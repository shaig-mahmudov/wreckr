package guardrails

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/wreckr/wreckr/apps/api/internal/config"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

type ViolationError struct {
	Problems []string
}

func (e ViolationError) Error() string {
	return "guardrail violation: " + strings.Join(e.Problems, "; ")
}

func Validate(sc scenario.Scenario, cfg config.Guardrails) error {
	var problems []string

	if cfg.MaxConcurrency > 0 && sc.Traffic.Concurrency > cfg.MaxConcurrency {
		problems = append(problems, fmt.Sprintf("traffic.concurrency %d exceeds max_concurrency %d", sc.Traffic.Concurrency, cfg.MaxConcurrency))
	}
	if cfg.MaxRequestRate > 0 && sc.Traffic.RatePerSecond > cfg.MaxRequestRate {
		problems = append(problems, fmt.Sprintf("traffic.rate_per_second %d exceeds max_request_rate_per_second %d", sc.Traffic.RatePerSecond, cfg.MaxRequestRate))
	}

	validateRequestBodies(&problems, "setup", sc.Setup, cfg.MaxRequestBodyBytes)
	validateRequestBodies(&problems, "requests", sc.Requests, cfg.MaxRequestBodyBytes)
	validateRequestBodies(&problems, "teardown", sc.Teardown, cfg.MaxRequestBodyBytes)

	baseURL, err := parseTargetURL(sc.Target.BaseURL)
	if err != nil {
		problems = append(problems, "target.base_url "+err.Error())
	} else {
		validateTargetURL(&problems, "target.base_url", baseURL, cfg.TargetAllowlist)
		validateRequestTargets(&problems, "setup", baseURL, sc.Setup, cfg.TargetAllowlist)
		validateRequestTargets(&problems, "requests", baseURL, sc.Requests, cfg.TargetAllowlist)
		validateRequestTargets(&problems, "teardown", baseURL, sc.Teardown, cfg.TargetAllowlist)
		validateInvariantTargets(&problems, baseURL, sc.Invariants, cfg.TargetAllowlist)
	}

	if len(problems) > 0 {
		return ViolationError{Problems: problems}
	}
	return nil
}

func validateRequestBodies(problems *[]string, prefix string, requests []scenario.Request, maxBytes int64) {
	if maxBytes <= 0 {
		return
	}
	for i, req := range requests {
		if size := int64(len(req.JSON)); size > maxBytes {
			*problems = append(*problems, fmt.Sprintf("%s[%d].json size %d exceeds max_request_body_bytes %d", prefix, i, size, maxBytes))
		}
		if size := int64(len(req.Body)); size > maxBytes {
			*problems = append(*problems, fmt.Sprintf("%s[%d].body size %d exceeds max_request_body_bytes %d", prefix, i, size, maxBytes))
		}
	}
}

func validateRequestTargets(problems *[]string, prefix string, baseURL *url.URL, requests []scenario.Request, allowlist []string) {
	for i, req := range requests {
		validatePathTarget(problems, fmt.Sprintf("%s[%d].path", prefix, i), baseURL, req.Path, allowlist)
	}
}

func validateInvariantTargets(problems *[]string, baseURL *url.URL, invariants []scenario.Invariant, allowlist []string) {
	for i, invariant := range invariants {
		if invariant.Type != "http_probe" {
			continue
		}
		validatePathTarget(problems, fmt.Sprintf("invariants[%d].path", i), baseURL, invariant.Path, allowlist)
	}
}

func validatePathTarget(problems *[]string, field string, baseURL *url.URL, rawPath string, allowlist []string) {
	targetURL, absolute, err := resolveURL(baseURL, rawPath)
	if err != nil {
		*problems = append(*problems, fmt.Sprintf("%s %s", field, err.Error()))
		return
	}
	validateTargetURL(problems, field, targetURL, allowlist)
	if absolute && len(allowlist) == 0 && !sameOrigin(baseURL, targetURL) {
		*problems = append(*problems, fmt.Sprintf("%s absolute URL host %q does not match target.base_url host %q; configure target allowlist to permit external targets", field, targetURL.Host, baseURL.Host))
	}
}

func validateTargetURL(problems *[]string, field string, targetURL *url.URL, allowlist []string) {
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		*problems = append(*problems, fmt.Sprintf("%s must use http or https", field))
	}
	if targetURL.Host == "" {
		*problems = append(*problems, fmt.Sprintf("%s must include a host", field))
	}
	if targetURL.User != nil {
		*problems = append(*problems, fmt.Sprintf("%s must not include credentials", field))
	}
	if strings.ContainsAny(targetURL.Host, " \t\r\n") {
		*problems = append(*problems, fmt.Sprintf("%s host must not contain whitespace", field))
	}
	if isMetadataHost(targetURL.Hostname()) && !matchesAllowlist(targetURL, allowlist) {
		*problems = append(*problems, fmt.Sprintf("%s host %q is blocked because it targets instance metadata", field, targetURL.Hostname()))
	}
	if len(allowlist) > 0 && !matchesAllowlist(targetURL, allowlist) {
		*problems = append(*problems, fmt.Sprintf("%s host %q is not in target allowlist", field, targetURL.Host))
	}
}

func parseTargetURL(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func resolveURL(baseURL *url.URL, rawPath string) (*url.URL, bool, error) {
	rel, err := url.Parse(rawPath)
	if err != nil {
		return nil, false, err
	}
	absolute := rel.IsAbs() || rel.Host != ""
	return baseURL.ResolveReference(rel), absolute, nil
}

func sameOrigin(left *url.URL, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}

func matchesAllowlist(targetURL *url.URL, allowlist []string) bool {
	host := strings.ToLower(targetURL.Hostname())
	hostport := strings.ToLower(targetURL.Host)
	for _, entry := range allowlist {
		pattern := normalizeAllowlistEntry(entry)
		if pattern == "" {
			continue
		}
		if pattern == "*" {
			return true
		}
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*.")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if strings.Contains(pattern, ":") {
			if pattern == hostport {
				return true
			}
			continue
		}
		if pattern == host {
			return true
		}
	}
	return false
}

func normalizeAllowlistEntry(entry string) string {
	entry = strings.ToLower(strings.TrimSpace(entry))
	if entry == "" {
		return ""
	}
	if strings.Contains(entry, "://") {
		if u, err := url.Parse(entry); err == nil && u.Host != "" {
			return strings.ToLower(u.Host)
		}
	}
	return strings.TrimSuffix(entry, "/")
}

func isMetadataHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "metadata.google.internal" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.Equal(net.ParseIP("169.254.169.254"))
}
