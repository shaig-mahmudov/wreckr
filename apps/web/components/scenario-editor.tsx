"use client";

import { AlertTriangle, CheckCircle2, Code2, Play, Save, Server, Split, WandSparkles } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { parse as parseYAML, stringify as stringifyYAML } from "yaml";
import { apiRequest, RunRecord, ScenarioDefinition, ScenarioRecord, ScenarioVersionRecord, sortVersions } from "../lib/scenario-api";
import { useAPIURL } from "../lib/use-api-url";

type EditorFormat = "json" | "yaml";

type ScenarioEditorProps = {
  initialScenario: ScenarioDefinition;
  scenarioID?: string;
  selectedVersionNumber?: number;
  versions?: ScenarioVersionRecord[];
  title: string;
  eyebrow: string;
  saveLabel: string;
};

type ValidationState =
  | { status: "idle"; messages: string[] }
  | { status: "valid"; messages: string[] }
  | { status: "invalid"; messages: string[] };

export function ScenarioEditor({ initialScenario, scenarioID, selectedVersionNumber, versions = [], title, eyebrow, saveLabel }: ScenarioEditorProps) {
  const router = useRouter();
  const [apiURL, setAPIURL] = useAPIURL();
  const [format, setFormat] = useState<EditorFormat>("json");
  const [scenarioText, setScenarioText] = useState(JSON.stringify(initialScenario, null, 2));
  const [dirty, setDirty] = useState(false);
  const [validation, setValidation] = useState<ValidationState>({ status: "idle", messages: [] });
  const [apiError, setAPIError] = useState<string | null>(null);
  const [latestRun, setLatestRun] = useState<RunRecord | null>(null);
  const [busyAction, setBusyAction] = useState<"save" | "run" | null>(null);

  const parsed = useMemo(() => parseScenario(scenarioText, format), [scenarioText, format]);
  const canSave = !busyAction;
  const canRun = Boolean(scenarioID && selectedVersionNumber) && !dirty && !busyAction;
  const sortedVersions = sortVersions(versions);

  function validateScenario() {
    const result = parseAndValidate();
    setValidation(result.ok ? { status: "valid", messages: ["Scenario is ready to save or run."] } : { status: "invalid", messages: result.messages });
    return result;
  }

  function parseAndValidate() {
    if (!parsed.ok) {
      return { ok: false as const, messages: [parsed.message] };
    }
    const messages = validateScenarioShape(parsed.value);
    if (messages.length > 0) {
      return { ok: false as const, messages };
    }
    return { ok: true as const, scenario: parsed.value };
  }

  async function saveScenario() {
    const result = validateScenario();
    if (!result.ok) {
      return;
    }
    setBusyAction("save");
    setAPIError(null);
    try {
      const record = scenarioID
        ? await apiRequest<ScenarioRecord>(apiURL, `/v1/scenarios/${scenarioID}`, {
            method: "PUT",
            body: JSON.stringify(result.scenario)
          })
        : await apiRequest<ScenarioRecord>(apiURL, "/v1/scenarios", {
            method: "POST",
            body: JSON.stringify(result.scenario)
          });
      router.push(`/scenarios/${record.id}`);
      router.refresh();
    } catch (err) {
      setAPIError(err instanceof Error ? err.message : "Could not save scenario.");
    } finally {
      setBusyAction(null);
    }
  }

  async function runScenarioVersion() {
    const result = validateScenario();
    if (!result.ok || !scenarioID || !selectedVersionNumber) {
      return;
    }
    setBusyAction("run");
    setAPIError(null);
    setLatestRun(null);
    try {
      const run = await apiRequest<RunRecord>(apiURL, "/v1/runs", {
        method: "POST",
        body: JSON.stringify({
          scenario_id: scenarioID,
          scenario_version_number: selectedVersionNumber,
          sync: false
        })
      });
      setLatestRun(run);
    } catch (err) {
      setAPIError(err instanceof Error ? err.message : "Could not start run.");
    } finally {
      setBusyAction(null);
    }
  }

  function switchFormat(nextFormat: EditorFormat) {
    if (nextFormat === format) {
      return;
    }
    if (parsed.ok) {
      setScenarioText(nextFormat === "json" ? JSON.stringify(parsed.value, null, 2) : stringifyYAML(parsed.value));
      setValidation({ status: "idle", messages: [] });
    }
    setFormat(nextFormat);
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">{eyebrow}</p>
          <h1>{title}</h1>
        </div>
        <nav className="nav-actions" aria-label="Dashboard navigation">
          <Link href="/">Console</Link>
          <Link href="/scenarios">Scenarios</Link>
        </nav>
      </header>

      <section className="editor-layout">
        <section className="scenario-panel" aria-label="Scenario definition editor">
          <div className="editor-toolbar">
            <div className="segmented-control" aria-label="Editor format">
              <button className={format === "json" ? "active" : ""} type="button" onClick={() => switchFormat("json")}>
                <Code2 size={16} />
                <span>JSON</span>
              </button>
              <button className={format === "yaml" ? "active" : ""} type="button" onClick={() => switchFormat("yaml")}>
                <Split size={16} />
                <span>YAML</span>
              </button>
            </div>
            <button className="text-button" type="button" onClick={validateScenario}>
              <CheckCircle2 size={16} />
              Validate
            </button>
          </div>
          <textarea
            spellCheck={false}
            value={scenarioText}
            onChange={(event) => {
              setScenarioText(event.target.value);
              setDirty(true);
            }}
            aria-label="Scenario definition"
          />
        </section>

        <aside className="editor-side" aria-label="Scenario actions">
          <section className="action-panel">
            <label>
              <span className="label">API</span>
              <input value={apiURL} onChange={(event) => setAPIURL(event.target.value)} />
            </label>
            <div className="action-grid">
              <button className="run-button" type="button" onClick={saveScenario} disabled={!canSave}>
                <Save size={18} />
                <span>{busyAction === "save" ? "Saving" : saveLabel}</span>
              </button>
              <button className="run-button secondary" type="button" onClick={runScenarioVersion} disabled={!canRun}>
                <Play size={18} />
                <span>{busyAction === "run" ? "Starting" : "Run Version"}</span>
              </button>
            </div>
            {scenarioID && selectedVersionNumber ? (
              <p className="helper-text">
                {dirty ? "Save changes as a new version before running." : `Run starts scenario version ${selectedVersionNumber}. Saving creates a new immutable version.`}
              </p>
            ) : (
              <p className="helper-text">Save the scenario before running a versioned execution.</p>
            )}
          </section>

          {validation.status !== "idle" ? (
            <Notice tone={validation.status === "valid" ? "good" : "bad"} messages={validation.messages} />
          ) : null}
          {apiError ? <Notice tone="bad" messages={[apiError]} /> : null}
          {latestRun ? (
            <section className="action-panel">
              <div className="section-title compact">
                <Server size={17} />
                <h2>Run Started</h2>
              </div>
              <dl className="detail-list">
                <div>
                  <dt>ID</dt>
                  <dd>{latestRun.id}</dd>
                </div>
                <div>
                  <dt>Status</dt>
                  <dd>{latestRun.status}</dd>
                </div>
                <div>
                  <dt>Version</dt>
                  <dd>{latestRun.scenario_version_number ?? selectedVersionNumber}</dd>
                </div>
              </dl>
              <Link className="text-link" href="/">
                Open run console
              </Link>
            </section>
          ) : null}

          <section className="action-panel">
            <div className="section-title compact">
              <WandSparkles size={17} />
              <h2>Versions</h2>
            </div>
            {scenarioID && sortedVersions.length > 0 ? (
              <div className="version-list">
                {sortedVersions.map((version) => (
                  <Link key={version.id} className={version.version_number === selectedVersionNumber ? "selected" : ""} href={`/scenarios/${scenarioID}/versions/${version.version_number}`}>
                    <strong>v{version.version_number}</strong>
                    <span>{new Date(version.created_at).toLocaleString()}</span>
                  </Link>
                ))}
              </div>
            ) : (
              <p className="empty-state">No saved versions yet</p>
            )}
          </section>
        </aside>
      </section>
    </main>
  );
}

function Notice({ tone, messages }: { tone: "good" | "bad"; messages: string[] }) {
  return (
    <div className={`notice ${tone}`}>
      {tone === "good" ? <CheckCircle2 size={18} /> : <AlertTriangle size={18} />}
      <ul>
        {messages.map((message) => (
          <li key={message}>{message}</li>
        ))}
      </ul>
    </div>
  );
}

function parseScenario(text: string, format: EditorFormat): { ok: true; value: ScenarioDefinition } | { ok: false; message: string } {
  try {
    const value = format === "json" ? JSON.parse(text) : parseYAML(text);
    if (!isRecord(value)) {
      return { ok: false, message: "Scenario must be an object." };
    }
    return { ok: true, value: value as ScenarioDefinition };
  } catch (err) {
    return { ok: false, message: err instanceof Error ? err.message : "Scenario definition is invalid." };
  }
}

function validateScenarioShape(sc: ScenarioDefinition) {
  const messages: string[] = [];
  if (!isNonEmptyString(sc.name)) {
    messages.push("name is required.");
  }
  if (!isRecord(sc.target) || !isNonEmptyString(sc.target.base_url)) {
    messages.push("target.base_url is required.");
  } else if (!/^https?:\/\//.test(sc.target.base_url)) {
    messages.push("target.base_url must start with http:// or https://.");
  }
  if (!isRecord(sc.traffic)) {
    messages.push("traffic is required.");
  }
  if (!Array.isArray(sc.requests) || sc.requests.length === 0) {
    messages.push("at least one request is required.");
  } else {
    sc.requests.forEach((request, index) => {
      if (!isRecord(request)) {
        messages.push(`requests[${index}] must be an object.`);
        return;
      }
      if (!isNonEmptyString(request.name)) {
        messages.push(`requests[${index}].name is required.`);
      }
      if (!isNonEmptyString(request.path)) {
        messages.push(`requests[${index}].path is required.`);
      }
    });
  }
  return messages;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}
