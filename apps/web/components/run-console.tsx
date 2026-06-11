"use client";

import { AlertTriangle, CheckCircle2, FileJson, Play, RefreshCw, ServerCrash, XCircle } from "lucide-react";
import { useMemo, useState } from "react";
import { checkoutRaceScenario, loginBurstScenario } from "../lib/sample-scenarios";

type RunReport = {
  id?: string;
  status?: string;
  report?: {
    status: "passed" | "failed";
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
  const [report, setReport] = useState<RunReport | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const parsedScenario = useMemo(() => {
    try {
      return JSON.parse(scenarioText);
    } catch {
      return null;
    }
  }, [scenarioText]);

  const statusClass = report?.report?.status === "passed" ? "good" : report?.report?.status === "failed" ? "bad" : "idle";
  const canRun = Boolean(parsedScenario) && !busy;

  function selectSample(id: string) {
    const sample = samples.find((item) => item.id === id) ?? samples[0];
    setSelectedSample(sample.id);
    setScenarioText(JSON.stringify(sample.value, null, 2));
    setReport(null);
    setError(null);
  }

  async function runScenario() {
    if (!parsedScenario) {
      setError("Scenario JSON is invalid.");
      return;
    }
    setBusy(true);
    setError(null);
    setReport(null);
    try {
      const response = await fetch(`${apiURL.replace(/\/$/, "")}/v1/runs`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ scenario: parsedScenario, sync: true })
      });
      const payload = (await response.json()) as RunReport;
      if (!response.ok) {
        throw new Error(payload.error ?? `Request failed with ${response.status}`);
      }
      setReport(payload);
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
          {report?.report?.status === "passed" ? <CheckCircle2 size={18} /> : report?.report?.status === "failed" ? <XCircle size={18} /> : <ServerCrash size={18} />}
          <span>{report?.report?.status ?? "ready"}</span>
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
            <Metric label="Requests" value={report?.report?.summary.total_requests ?? "-"} />
            <Metric label="Failed" value={report?.report?.summary.failed_requests ?? "-"} />
            <Metric label="Error Rate" value={report ? `${((report.report?.summary.error_rate ?? 0) * 100).toFixed(2)}%` : "-"} />
            <Metric label="p95" value={report ? `${(report.report?.summary.latency.p95_ms ?? 0).toFixed(1)}ms` : "-"} />
          </div>

          <section className="failure-list" aria-label="Failures">
            <div className="section-title">
              <XCircle size={17} />
              <h2>Failures</h2>
            </div>
            {report?.report?.failures?.length ? (
              <ul>
                {report.report.failures.map((failure) => (
                  <li key={failure}>{failure}</li>
                ))}
              </ul>
            ) : (
              <p>{report ? "None" : "No run yet"}</p>
            )}
          </section>

          <section className="json-report" aria-label="Raw report">
            <div className="section-title">
              <FileJson size={17} />
              <h2>Report</h2>
            </div>
            <pre>{report ? JSON.stringify(report, null, 2) : "{}"}</pre>
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
