# Run Guardrails

Wreckr is a load and pressure testing tool designed to generate extreme traffic. To prevent accidental denial of service attacks, high infrastructure costs, server-side request forgery (SSRF), or cloud metadata exposure, Wreckr implements a robust **Guardrails Validation Layer**.

Before any scenario is stored or executed, the API validates it against a set of hard safety limits and network policies.

## Configuration

Guardrails are configured using environment variables in the API process. The default limits are conservative and suited for local sandbox testing.

| Environment Variable | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `WRECKR_MAX_CONCURRENCY` | Integer | `1000` | The maximum virtual users (VUs) allowed in a scenario's traffic block. |
| `WRECKR_MAX_REQUEST_RATE_PER_SECOND` | Integer | `5000` | The maximum request-rate pacing limit allowed across all requests. |
| `WRECKR_MAX_RUN_DURATION_SECONDS` | Integer | `300` (5 mins) | The maximum run duration limit for any scenario. |
| `WRECKR_MAX_REQUEST_BODY_BYTES` | Integer (Bytes) | `1048576` (1MB) | The maximum allowed request payload/body size in bytes. |
| `WRECKR_TARGET_ALLOWLIST` | CSV String | `""` (Any) | Comma-separated list of allowed domains or IPs. If set, any other target is rejected. |
| `WRECKR_ALLOW_METADATA_TARGETS` | Boolean | `false` | If false, explicitly blocks internal cloud metadata endpoints (e.g. AWS Instance Metadata Service). |

## Validation Checks

When a scenario is submitted to `POST /v1/runs` or `POST /v1/scenarios`, the `internal/guardrails` package runs the following assertions:

### 1. Traffic Limits
- **Concurrency Cap:** Rejects runs with `traffic.concurrency` exceeding `WRECKR_MAX_CONCURRENCY`.
- **RPS/Pacing Cap:** Rejects runs with `traffic.rate_per_second` exceeding `WRECKR_MAX_REQUEST_RATE_PER_SECOND`.
- **Duration Cap:** Rejects runs where expected duration exceeds `WRECKR_MAX_RUN_DURATION_SECONDS`.

### 2. Request Limits
- **Body Size Limit:** Any request in `setup`, `requests`, or `teardown` with a `body` or `json` size exceeding `WRECKR_MAX_REQUEST_BODY_BYTES` is rejected early to avoid worker heap exhaustion.

### 3. Target and Network Security
- **Domain Allowlist:** If `WRECKR_TARGET_ALLOWLIST` is configured (e.g., `api.staging.internal,*.demo.com`), Wreckr parses the target URL's host. If the host doesn't match the list or wildcard patterns, the run is rejected.
- **Unsafe Protocol/URL Rejection:** Only `http://` and `https://` URLs are permitted. Relative URL schemes are resolved through the Target configuration. Malformed or absolute paths with hidden/backdoor protocols (e.g., `file://`, `gopher://`) are immediately blocked.
- **Metadata Protection:** AWS IMDS (`169.254.169.254`) and other typical cloud provider metadata endpoints are blocked by default to prevent server-side request forgery (SSRF) where an attacker attempts to exfiltrate cloud credentials through Wreckr. If `WRECKR_ALLOW_METADATA_TARGETS` is set to `true`, this protection is bypassed (useful for specialized internal network testing).

## Error Response

If a guardrail validation fails, the API returns a structured HTTP `400 Bad Request` or `422 Unprocessable Entity` containing a clear error message describing which guardrail limit was violated:

```json
{
  "error": "guardrail violation: traffic concurrency (1500) exceeds maximum allowed (1000)"
}
```
