"use client";

import { Activity, AlertTriangle, CheckCircle2, FileJson, ListChecks, Play, RefreshCw, ServerCrash, XCircle } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { checkoutRaceScenario, loginBurstScenario } from "../lib/sample-scenarios";

type WreckrReport = {
  status: "passed" | "failed" | "canceled";
  scenario: string;
  duration_ms: number;
  summary: {
    total_requests: number;
    failed_requests: number;
    error_rate: number;
    status_codes: Record<string, number>;
    latency: {
      p50_ms: number;
      p95_ms: number;
      p99_ms: number;
    };
  };
  failures?: string[];
  invariants?: Array<{ name: string; passed: boolean; message: string }>;
};

type RunRecord = {
  id: string;
  scenario_id?: string;
  status: "queued" | "running" | "passed" | "failed" | "errored" | "canceled";
  scenario?: {
    name?: string;
  };
  created_at?: string;
  started_at?: string;
  finished_at?: string;
  error?: string;
  report?: WreckrReport;
};

type RunResponse = RunRecord & {
  error?: string;
};

type RunsResponse = {
  runs?: RunRecord[];
  error?: string;
};

const samples = [
  { id: "checkout", label: "Checkout Race", value: checkoutRaceScenario },
  { id: "login", label: "Login Burst", value: loginBurstScenario }
];

export function RunConsole() {
  const [apiURL, setAPIURL] = useState(process.env.NEXT_PUBLIC_WRECKR_API_URL ?? "http://localhost:8080");
  const [selectedSample, setSelectedSample] = useState(samples[0].id);
  const [scenarioText, setScenarioText] = useState(JSON.stringify(samples[0].value, null, 2));
  const [latestRun, setLatestRun] = useState<RunResponse | null>(null);
  const [runs, setRuns] = useState<RunRecord[]>([]);
  const [selectedRunID, setSelectedRunID] = useState<string | null>(null);
  const [apiState, setAPIState] = useState<"checking" | "connected" | "offline">("checking");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const parsedScenario = useMemo(() => {
    try {
      return JSON.parse(scenarioText);
    } catch {
      return null;
    }
  }, [scenarioText]);

  const selectedRun = runs.find((run) => run.id === selectedRunID) ?? latestRun;
  const activeReport = selectedRun?.report;
  const statusClass = activeReport?.status === "passed" ? "good" : activeReport?.status === "failed" ? "bad" : "idle";
  const canRun = Boolean(parsedScenario) && !busy;

  useEffect(() => {
    void refreshRuns();
  }, []);

  function selectSample(id: string) {
    const sample = samples.find((item) => item.id === id) ?? samples[0];
    setSelectedSample(sample.id);
    setScenarioText(JSON.stringify(sample.value, null, 2));
    setLatestRun(null);
    setSelectedRunID(null);
    setError(null);
  }

  async function refreshRuns(preferredRunID?: string) {
    setAPIState("checking");
    setError(null);
    try {
      const baseURL = apiURL.replace(/\/$/, "");
      const health = await fetch(`${baseURL}/healthz`, { cache: "no-store" });
      if (!health.ok) {
        throw new Error(`API health check failed with ${health.status}`);
      }
      const response = await fetch(`${baseURL}/v1/runs`, { cache: "no-store" });
      const payload = (await response.json()) as RunsResponse;
      if (!response.ok) {
        throw new Error(payload.error ?? `Run list failed with ${response.status}`);
      }
      const nextRuns = [...(payload.runs ?? [])].sort((a, b) => Date.parse(b.created_at ?? "") - Date.parse(a.created_at ?? ""));
      setRuns(nextRuns);
      setAPIState("connected");
      if (preferredRunID) {
        setSelectedRunID(preferredRunID);
      } else if (!selectedRunID && nextRuns.length > 0) {
        setSelectedRunID(nextRuns[0].id);
      }
    } catch (err) {
      setAPIState("offline");
      setRuns([]);
      setError(err instanceof Error ? err.message : "Could not connect to the Wreckr API.");
    }
  }

  async function runScenario() {
    if (!parsedScenario) {
      setError("Scenario JSON is invalid.");
      return;
    }
    setBusy(true);
    setError(null);
    setLatestRun(null);
    try {
      const response = await fetch(`${apiURL.replace(/\/$/, "")}/v1/runs`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ scenario: parsedScenario, sync: false })
      });
      const payload = (await response.json()) as RunResponse;
      if (!response.ok) {
        throw new Error(payload.error ?? `Request failed with ${response.status}`);
      }
      setLatestRun(payload);
      setSelectedRunID(payload.id);
      await refreshRuns(payload.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Run failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Wreckr</p>
          <h1>Production Scenario Console</h1>
        </div>
        <div className={`run-state ${statusClass}`}>
          {activeReport?.status === "passed" ? <CheckCircle2 size={18} /> : activeReport?.status === "failed" ? <XCircle size={18} /> : <ServerCrash size={18} />}
          <span>{activeReport?.status ?? "ready"}</span>
        </div>
      </header>

      <section className="workspace">
        <section className="scenario-panel" aria-label="Scenario editor">
          <div className="panel-head">
            <div>
              <span className="label">Scenario</span>
              <h2>{parsedScenario?.name ?? "Invalid JSON"}</h2>
            </div>
            <div className="sample-tabs" aria-label="Sample scenarios">
              {samples.map((sample) => (
                <button
                  key={sample.id}
                  className={sample.id === selectedSample ? "active" : ""}
                  type="button"
                  onClick={() => selectSample(sample.id)}
                >
                  {sample.label}
                </button>
              ))}
            </div>
          </div>
          <textarea
            spellCheck={false}
            value={scenarioText}
            onChange={(event) => setScenarioText(event.target.value)}
            aria-label="Scenario JSON"
          />
        </section>

        <section className="result-panel" aria-label="Run report">
          <div className="toolbar">
            <label>
              <span>API</span>
              <input value={apiURL} onChange={(event) => setAPIURL(event.target.value)} />
            </label>
            <div className={`api-pill ${apiState}`}>
              <Activity size={16} />
              <span>{apiState}</span>
            </div>
            <button className="icon-button" type="button" onClick={() => selectSample(selectedSample)} title="Reload sample">
              <RefreshCw size={18} />
            </button>
            <button className="run-button" type="button" onClick={runScenario} disabled={!canRun}>
              <Play size={18} />
              <span>{busy ? "Running" : "Run"}</span>
            </button>
          </div>

          {error ? (
            <div className="notice bad">
              <AlertTriangle size={18} />
              <span>{error}</span>
            </div>
          ) : null}

          <div className="metric-grid">
            <Metric label="Requests" value={activeReport?.summary.total_requests ?? "-"} />
            <Metric label="Failed" value={activeReport?.summary.failed_requests ?? "-"} />
            <Metric label="Error Rate" value={activeReport ? `${(activeReport.summary.error_rate * 100).toFixed(2)}%` : "-"} />
            <Metric label="p95" value={activeReport ? `${activeReport.summary.latency.p95_ms.toFixed(1)}ms` : "-"} />
          </div>

          <section className="run-list" aria-label="Run list">
            <div className="section-title">
              <ListChecks size={17} />
              <h2>Runs</h2>
              <button className="text-button" type="button" onClick={() => refreshRuns()}>
                Refresh
              </button>
            </div>
            {runs.length > 0 ? (
              <div className="run-items">
                {runs.map((run) => (
                  <button
                    key={run.id}
                    className={run.id === selectedRun?.id ? "run-item selected" : "run-item"}
                    type="button"
                    onClick={() => setSelectedRunID(run.id)}
                  >
                    <span>
                      <strong>{run.scenario?.name ?? run.report?.scenario ?? "Untitled scenario"}</strong>
                      <small>{run.id}</small>
                    </span>
                    <em className={run.status}>{run.status}</em>
                  </button>
                ))}
              </div>
            ) : (
              <p className="empty-state">{apiState === "connected" ? "No runs yet" : "Connect to the API to load runs"}</p>
            )}
          </section>

          <section className="failure-list" aria-label="Failures">
            <div className="section-title">
              <XCircle size={17} />
              <h2>Failures</h2>
            </div>
            {activeReport?.failures?.length ? (
              <ul>
                {activeReport.failures.map((failure) => (
                  <li key={failure}>{failure}</li>
                ))}
              </ul>
            ) : (
              <p>{activeReport ? "None" : "No report selected"}</p>
            )}
          </section>

          <section className="json-report" aria-label="Raw report">
            <div className="section-title">
              <FileJson size={17} />
              <h2>Report</h2>
            </div>
            <pre>{selectedRun ? JSON.stringify(selectedRun.report ?? selectedRun, null, 2) : "{}"}</pre>
          </section>
        </section>
      </section>
    </main>
  );
}

function Metric({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
