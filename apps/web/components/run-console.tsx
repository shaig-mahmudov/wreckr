"use client";

import { Activity, AlertTriangle, CheckCircle2, FileJson, ListChecks, Play, RefreshCw, ServerCrash, XCircle } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { checkoutRaceScenario, loginBurstScenario } from "../lib/sample-scenarios";
import { useAPIURL } from "../lib/use-api-url";
import { TargetManager, type TargetRecord } from "./target-manager";


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
  target_id?: string;
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

type RunEvent = {
  id?: string;
  run_id: string;
  sequence: number;
  level: "info" | "warn" | "error";
  type: string;
  message: string;
  metadata?: Record<string, unknown>;
  created_at: string;
};

type EventsResponse = {
  events?: RunEvent[];
  error?: string;
};

const samples = [
  { id: "checkout", label: "Checkout Race", value: checkoutRaceScenario },
  { id: "login", label: "Login Burst", value: loginBurstScenario }
];

const streamEventTypes = [
  "run_queued",
  "run_started",
  "setup_started",
  "setup_completed",
  "teardown_started",
  "teardown_completed",
  "request_started",
  "request_completed",
  "assertion_failed",
  "invariant_failed",
  "threshold_failed",
  "cancel_requested",
  "run_completed",
  "run_failed",
  "run_canceled"
];

const terminalEventTypes = new Set(["run_completed", "run_failed", "run_canceled"]);

export function RunConsole() {
  const [apiURL, setAPIURL] = useAPIURL();
  const [selectedSample, setSelectedSample] = useState(samples[0].id);
  const [scenarioText, setScenarioText] = useState(JSON.stringify(samples[0].value, null, 2));
  const [latestRun, setLatestRun] = useState<RunResponse | null>(null);
  const [runs, setRuns] = useState<RunRecord[]>([]);
  const [runEvents, setRunEvents] = useState<RunEvent[]>([]);
  const [selectedRunID, setSelectedRunID] = useState<string | null>(null);
  const [apiState, setAPIState] = useState<"checking" | "connected" | "offline">("checking");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const [targets, setTargets] = useState<TargetRecord[]>([]);
  const [selectedTargetID, setSelectedTargetID] = useState<string>("");

  const parsedScenario = useMemo(() => {
    try {
      return JSON.parse(scenarioText);
    } catch {
      return null;
    }
  }, [scenarioText]);

  const selectedRun = runs.find((run) => run.id === selectedRunID) ?? latestRun;
  const activeReport = selectedRun?.report;
  const activeStatus = activeReport?.status ?? selectedRun?.status ?? "ready";
  const statusClass = activeStatus === "passed" ? "good" : activeStatus === "failed" || activeStatus === "errored" || activeStatus === "canceled" ? "bad" : "idle";
  const canRun = Boolean(parsedScenario) && !busy;

  useEffect(() => {
    void refreshRuns();
  }, [apiURL]);

  useEffect(() => {
    if (!selectedRunID) {
      setRunEvents([]);
      return;
    }

    const runID = selectedRunID;
    let closed = false;
    let source: EventSource | null = null;
    const baseURL = apiURL.replace(/\/$/, "");

    async function connect() {
      try {
        const response = await fetch(`${baseURL}/v1/runs/${runID}/events`, { cache: "no-store" });
        const payload = (await response.json()) as EventsResponse;
        if (!response.ok) {
          throw new Error(payload.error ?? `Event list failed with ${response.status}`);
        }
        const persistedEvents = payload.events ?? [];
        setRunEvents(persistedEvents);
        if (closed || persistedEvents.some((event) => terminalEventTypes.has(event.type))) {
          return;
        }

        source = new EventSource(`${baseURL}/v1/runs/${runID}/events/stream`);
        for (const eventType of streamEventTypes) {
          source.addEventListener(eventType, (message) => {
            const event = JSON.parse((message as MessageEvent).data) as RunEvent;
            appendRunEvent(event);
            if (terminalEventTypes.has(event.type)) {
              source?.close();
              void refreshRuns(runID);
            }
          });
        }
        source.onerror = () => {
          if (closed) {
            source?.close();
          }
        };
      } catch (err) {
        if (!closed) {
          setError(err instanceof Error ? err.message : "Could not load run events.");
        }
      }
    }

    void connect();
    return () => {
      closed = true;
      source?.close();
    };
  }, [apiURL, selectedRunID]);

  async function refreshTargets() {
    try {
      const baseURL = apiURL.replace(/\/$/, "");
      const response = await fetch(`${baseURL}/v1/targets`, { cache: "no-store" });
      const payload = await response.json();
      if (response.ok) {
        setTargets(payload.targets ?? []);
      }
    } catch (err) {
      // Ignore errors here, refreshRuns handles connection state
    }
  }

  function selectSample(id: string) {
    const sample = samples.find((item) => item.id === id) ?? samples[0];
    setSelectedSample(sample.id);
    setScenarioText(JSON.stringify(sample.value, null, 2));
    setLatestRun(null);
    setRunEvents([]);
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
    void refreshTargets();
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
        body: JSON.stringify({ 
          scenario: parsedScenario, 
          sync: false,
          target_id: selectedTargetID || undefined 
        })
      });
      const payload = (await response.json()) as RunResponse;
      if (!response.ok) {
        throw new Error(payload.error ?? `Request failed with ${response.status}`);
      }
      setLatestRun(payload);
      setRunEvents([]);
      setSelectedRunID(payload.id);
      await refreshRuns(payload.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Run failed.");
    } finally {
      setBusy(false);
    }
  }

  function appendRunEvent(event: RunEvent) {
    setRunEvents((current) => {
      const next = current.filter((item) => item.sequence !== event.sequence);
      next.push(event);
      next.sort((a, b) => a.sequence - b.sequence);
      return next;
    });
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Wreckr</p>
          <h1>Production Scenario Console</h1>
        </div>
        <div className="topbar-actions">
          <nav className="nav-actions" aria-label="Dashboard navigation">
            <Link href="/scenarios">Scenarios</Link>
          </nav>
          <div className={`run-state ${statusClass}`}>
            {activeReport?.status === "passed" ? <CheckCircle2 size={18} /> : activeReport?.status === "failed" ? <XCircle size={18} /> : <ServerCrash size={18} />}
            <span>{activeStatus}</span>
          </div>
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
            <label style={{maxWidth: 200}}>
              <span>Target</span>
              <select 
                value={selectedTargetID} 
                onChange={(e) => setSelectedTargetID(e.target.value)}
                style={{height: 40, padding: '0 12px', border: '1px solid var(--line)', borderRadius: 8, background: 'var(--surface)'}}
              >
                <option value="">Default (From Scenario)</option>
                {targets.map(t => (
                  <option key={t.id} value={t.id}>{t.name} ({t.environment})</option>
                ))}
              </select>
            </label>
            <button className="icon-button" type="button" onClick={() => { selectSample(selectedSample); void refreshTargets(); }} title="Reload sample">
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

          <section className="event-timeline" aria-label="Run event timeline">
            <div className="section-title">
              <Activity size={17} />
              <h2>Timeline</h2>
            </div>
            {runEvents.length > 0 ? (
              <ol>
                {runEvents.map((event) => (
                  <li key={`${event.run_id}-${event.sequence}`} className={event.level}>
                    <span>{event.sequence}</span>
                    <div>
                      <strong>{event.type}</strong>
                      <small>{event.message || new Date(event.created_at).toLocaleTimeString()}</small>
                    </div>
                  </li>
                ))}
              </ol>
            ) : (
              <p>{selectedRun ? "Waiting for events" : "No run selected"}</p>
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

      <div style={{marginTop: 40}}>
        <div className="workspace" style={{display: 'block'}}>
          <TargetManager apiURL={apiURL} />
        </div>
      </div>
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
