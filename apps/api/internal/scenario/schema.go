package scenario

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

const CurrentVersion = 1

type TrafficType string

const (
	TrafficLoad       TrafficType = "load"
	TrafficBurst      TrafficType = "burst"
	TrafficSpike      TrafficType = "spike"
	TrafficRace       TrafficType = "race"
	TrafficRetryStorm TrafficType = "retry_storm"
)

type Scenario struct {
	Version     int         `json:"version"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Target      Target      `json:"target"`
	Traffic     Traffic     `json:"traffic"`
	Setup       []Request   `json:"setup,omitempty"`
	Requests    []Request   `json:"requests"`
	Teardown    []Request   `json:"teardown,omitempty"`
	Thresholds  Thresholds  `json:"thresholds,omitempty"`
	Invariants  []Invariant `json:"invariants,omitempty"`
}

type Target struct {
	BaseURL string            `json:"base_url"`
	Headers map[string]string `json:"headers,omitempty"`
}

type Traffic struct {
	Type        TrafficType `json:"type"`
	Concurrency int         `json:"concurrency"`
	Iterations  int         `json:"iterations"`
	Retry       RetryPolicy `json:"retry,omitempty"`
}

type RetryPolicy struct {
	Attempts  int `json:"attempts,omitempty"`
	BackoffMS int `json:"backoff_ms,omitempty"`
}

type Request struct {
	Name    string             `json:"name"`
	Method  string             `json:"method"`
	Path    string             `json:"path"`
	Headers map[string]string  `json:"headers,omitempty"`
	JSON    json.RawMessage    `json:"json,omitempty"`
	Body    string             `json:"body,omitempty"`
	Expect  RequestExpectation `json:"expect,omitempty"`
}

type RequestExpectation struct {
	Status []int `json:"status,omitempty"`
}

type Thresholds struct {
	MaxErrorRate *float64 `json:"max_error_rate,omitempty"`
	P95MS        *float64 `json:"p95_ms,omitempty"`
}

type Invariant struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Request string            `json:"request,omitempty"`
	Method  string            `json:"method,omitempty"`
	Path    string            `json:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Status  *int              `json:"status,omitempty"`
	Equals  *int              `json:"equals,omitempty"`
	Min     *int              `json:"min,omitempty"`
	Max     *int              `json:"max,omitempty"`
	Expect  ProbeExpectation  `json:"expect,omitempty"`
}

type ProbeExpectation struct {
	JSONPath string `json:"json_path,omitempty"`
	Equals   any    `json:"equals,omitempty"`
}

func LoadFile(path string) (Scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, err
	}

	var sc Scenario
	if err := json.Unmarshal(raw, &sc); err != nil {
		return Scenario{}, fmt.Errorf("parse scenario: %w", err)
	}

	sc = sc.WithEnv()
	sc = sc.Normalized()
	if err := sc.Validate(); err != nil {
		return Scenario{}, err
	}

	return sc, nil
}

func (s Scenario) Normalized() Scenario {
	if s.Version == 0 {
		s.Version = CurrentVersion
	}
	if s.Traffic.Type == "" {
		s.Traffic.Type = TrafficLoad
	}
	if s.Traffic.Concurrency <= 0 {
		s.Traffic.Concurrency = 1
	}
	if s.Traffic.Iterations <= 0 {
		s.Traffic.Iterations = 1
	}
	if s.Traffic.Type == TrafficRetryStorm && s.Traffic.Retry.Attempts <= 0 {
		s.Traffic.Retry.Attempts = 3
	}
	if s.Target.Headers == nil {
		s.Target.Headers = map[string]string{}
	}
	normalizeRequests(s.Setup)
	normalizeRequests(s.Requests)
	normalizeRequests(s.Teardown)
	for i := range s.Invariants {
		if s.Invariants[i].Method == "" {
			s.Invariants[i].Method = "GET"
		}
		s.Invariants[i].Method = strings.ToUpper(s.Invariants[i].Method)
		if s.Invariants[i].Headers == nil {
			s.Invariants[i].Headers = map[string]string{}
		}
	}
	return s
}

func (s Scenario) WithEnv() Scenario {
	s.Name = os.ExpandEnv(s.Name)
	s.Description = os.ExpandEnv(s.Description)
	s.Target.BaseURL = os.ExpandEnv(s.Target.BaseURL)
	for k, v := range s.Target.Headers {
		s.Target.Headers[k] = os.ExpandEnv(v)
	}
	expandRequestEnv(s.Setup)
	expandRequestEnv(s.Requests)
	expandRequestEnv(s.Teardown)
	for i := range s.Invariants {
		s.Invariants[i].Name = os.ExpandEnv(s.Invariants[i].Name)
		s.Invariants[i].Request = os.ExpandEnv(s.Invariants[i].Request)
		s.Invariants[i].Path = os.ExpandEnv(s.Invariants[i].Path)
		for k, v := range s.Invariants[i].Headers {
			s.Invariants[i].Headers[k] = os.ExpandEnv(v)
		}
	}
	return s
}

func (s Scenario) Validate() error {
	var problems []string
	if s.Version != CurrentVersion {
		problems = append(problems, fmt.Sprintf("version must be %d", CurrentVersion))
	}
	if strings.TrimSpace(s.Name) == "" {
		problems = append(problems, "name is required")
	}
	if _, err := parseHTTPURL(s.Target.BaseURL); err != nil {
		problems = append(problems, "target.base_url "+err.Error())
	}
	if len(s.Requests) == 0 {
		problems = append(problems, "at least one request is required")
	}
	switch s.Traffic.Type {
	case TrafficLoad, TrafficBurst, TrafficSpike, TrafficRace, TrafficRetryStorm:
	default:
		problems = append(problems, "traffic.type must be one of load, burst, spike, race, retry_storm")
	}
	if s.Traffic.Concurrency < 1 {
		problems = append(problems, "traffic.concurrency must be >= 1")
	}
	if s.Traffic.Iterations < 1 {
		problems = append(problems, "traffic.iterations must be >= 1")
	}
	if s.Traffic.Retry.Attempts < 0 {
		problems = append(problems, "traffic.retry.attempts must be >= 0")
	}
	if s.Traffic.Retry.BackoffMS < 0 {
		problems = append(problems, "traffic.retry.backoff_ms must be >= 0")
	}
	validateRequests := func(prefix string, requests []Request) {
		for i, req := range requests {
			if strings.TrimSpace(req.Name) == "" {
				problems = append(problems, fmt.Sprintf("%s[%d].name is required", prefix, i))
			}
			if req.Path == "" {
				problems = append(problems, fmt.Sprintf("%s[%d].path is required", prefix, i))
			}
			if len(req.JSON) > 0 && !json.Valid(req.JSON) {
				problems = append(problems, fmt.Sprintf("%s[%d].json must be valid JSON", prefix, i))
			}
		}
	}
	validateRequests("setup", s.Setup)
	validateRequests("requests", s.Requests)
	validateRequests("teardown", s.Teardown)
	for i, inv := range s.Invariants {
		if strings.TrimSpace(inv.Name) == "" {
			problems = append(problems, fmt.Sprintf("invariants[%d].name is required", i))
		}
		switch inv.Type {
		case "response_count":
			if inv.Equals == nil && inv.Min == nil && inv.Max == nil {
				problems = append(problems, fmt.Sprintf("invariants[%d] response_count needs equals, min, or max", i))
			}
		case "http_probe":
			if inv.Path == "" {
				problems = append(problems, fmt.Sprintf("invariants[%d] http_probe needs path", i))
			}
			if inv.Expect.JSONPath == "" {
				problems = append(problems, fmt.Sprintf("invariants[%d] http_probe needs expect.json_path", i))
			}
		default:
			problems = append(problems, fmt.Sprintf("invariants[%d].type must be response_count or http_probe", i))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func normalizeRequests(requests []Request) {
	for i := range requests {
		if requests[i].Method == "" {
			requests[i].Method = "GET"
		}
		requests[i].Method = strings.ToUpper(requests[i].Method)
		if requests[i].Headers == nil {
			requests[i].Headers = map[string]string{}
		}
	}
}

func expandRequestEnv(requests []Request) {
	for i := range requests {
		requests[i].Name = os.ExpandEnv(requests[i].Name)
		requests[i].Path = os.ExpandEnv(requests[i].Path)
		requests[i].Body = os.ExpandEnv(requests[i].Body)
		if len(requests[i].JSON) > 0 {
			requests[i].JSON = json.RawMessage(os.ExpandEnv(string(requests[i].JSON)))
		}
		for k, v := range requests[i].Headers {
			requests[i].Headers[k] = os.ExpandEnv(v)
		}
	}
}

func parseHTTPURL(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("must use http or https")
	}
	if u.Host == "" {
		return nil, errors.New("must include a host")
	}
	return u, nil
}
