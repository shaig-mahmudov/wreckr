# k6 Compiler & Runner Execution

While Wreckr contains a lightweight in-process HTTP scenario runner in Go (perfect for quick local feedback or zero-dependency setups), high-scale performance, load, and concurrency benchmarks require specialized execution engines.

Wreckr features a **Deterministic k6 Compiler and Runner (`K6Runner`)** that allows compiling standard Wreckr JSON scenario contracts into highly efficient JavaScript k6 scripts, running them via the k6 engine, and importing results back into Wreckr.

## How It Works

Wreckr separates script compilation from execution to provide maximum transparency and flexibility:

```
┌─────────────────┐             ┌───────────────┐             ┌──────────────────┐
│ Wreckr Scenario │ ──Compile──►│ k6 JS Script  │ ────Run────►│ k6 JSON Summary  │
│     (JSON)      │             │ (Temp File)   │             │   (Temp File)    │
└─────────────────┘             └───────────────┘             └────────┬─────────┘
                                                                       │
┌─────────────────┐                                                    │
│  Wreckr Report  │◄──────────── Reconstruct Records ──────────────────┘
│  (Normalized)   │             (Regex status tags)
└─────────────────┘
```

1. **Compilation (`k6script` package):** Converts the scenario schema (including base URL, setup hooks, teardown hooks, traffic shapes, requests list, expected statuses, headers, and bodies) into a standard ESM JavaScript k6 script.
2. **Execution (`runner.K6Runner`):** Writes the generated script to a temporary file, invokes `k6 run --summary-export <tempSummary.json> <tempScript.js>`, and monitors execution.
3. **Data Import and Normalization:**
   - Reads the exported k6 summary JSON.
   - Extracts performance metrics such as latency distributions (min, max, average, p50, p95, p99) from `http_req_duration`.
   - Reconstructs specific Wreckr response records by parsing k6's status tag metrics using regex (e.g. `http_reqs{name:<name>,status:<code>}`).
   - Runs advanced HTTP probe invariants from the Go controller.
   - Merges metrics and outputs a unified `report.Report` that matches the identical schema returned by the native Go runner.

## Compilation Mapping

The compiler maps Wreckr scenario definitions to k6-native constructs:

### 1. Traffic Shapes
- **Load, Burst, Spike, Race:** Mapped to k6's `shared-iterations` executor with custom VUs and calculated iterations.
- **Retry Storm:** Mapped to a looping JavaScript script block that retries requests up to `attempts` times, pausing for `BackoffMS` using k6's `sleep()` helper.
- **Rate-Per-Second Limit:** Mapped directly to k6's global options property `rps`.

### 2. Request Handling
- Request paths, methods, JSON payloads, raw bodies, and custom headers are converted into corresponding `http.request()` blocks.
- Expected HTTP statuses are translated into k6's `check()` assertions.

## CLI Usage

You can compile a scenario to an inspectable k6 script manually using the CLI:

```bash
wreckr compile-k6 -o my-scenario.js ./scenarios/my-test.json
```

You can then run the script with your local k6 installation:

```bash
k6 run my-scenario.js
```

## API Configuration

To instruct the async worker to use the k6 runner instead of the default Go engine, set the runner engine configuration:

```bash
WRECKR_RUNNER_ENGINE=k6
```

When this is set, any API or dashboard run triggers the `K6Runner` in the background worker process. The worker executes the compiled scenario via k6, reads the performance summary, evaluates custom invariants from Go, and persists the normalized Wreckr report seamlessly.
