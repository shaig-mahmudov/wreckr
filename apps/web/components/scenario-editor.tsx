"use client";

import { AlertTriangle, CheckCircle2, Code2, Play, Save, Server, Split, WandSparkles, Trash2, Plus, ChevronDown, ChevronUp } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { parse as parseYAML, stringify as stringifyYAML } from "yaml";
import { apiRequest, RunRecord, ScenarioDefinition, ScenarioRecord, ScenarioVersionRecord, sortVersions } from "../lib/scenario-api";
import { useAPIURL } from "../lib/use-api-url";

type EditorFormat = "visual" | "json" | "yaml";

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
  const [format, setFormat] = useState<EditorFormat>("visual");
  const [scenarioText, setScenarioText] = useState(JSON.stringify(initialScenario, null, 2));
  const [scenarioObj, setScenarioObj] = useState<ScenarioDefinition>(initialScenario);
  const [dirty, setDirty] = useState(false);
  const [validation, setValidation] = useState<ValidationState>({ status: "idle", messages: [] });
  const [apiError, setAPIError] = useState<string | null>(null);
  const [latestRun, setLatestRun] = useState<RunRecord | null>(null);
  const [busyAction, setBusyAction] = useState<"save" | "run" | null>(null);
  const [expandedRequests, setExpandedRequests] = useState<Record<number, boolean>>({ 0: true });

  const parsed = useMemo(() => {
    if (format === "visual") {
      return { ok: true as const, value: scenarioObj };
    }
    return parseScenario(scenarioText, format);
  }, [scenarioText, format, scenarioObj]);

  const canSave = !busyAction;
  const canRun = Boolean(scenarioID && selectedVersionNumber) && !dirty && !busyAction;
  const sortedVersions = sortVersions(versions);

  const updateScenarioObj = (newObj: ScenarioDefinition) => {
    setScenarioObj(newObj);
    setScenarioText(JSON.stringify(newObj, null, 2));
    setDirty(true);
  };

  function validateScenario() {
    const result = parseAndValidate();
    setValidation(result.ok ? { status: "valid", messages: ["Scenario is ready to save or run."] } : { status: "invalid", messages: result.messages });
    return result;
  }

  function parseAndValidate() {
    if (format === "visual") {
      const messages = validateScenarioShape(scenarioObj);
      if (messages.length > 0) {
        return { ok: false as const, messages };
      }
      return { ok: true as const, scenario: scenarioObj };
    } else {
      if (!parsed.ok) {
        return { ok: false as const, messages: [parsed.message] };
      }
      const messages = validateScenarioShape(parsed.value);
      if (messages.length > 0) {
        return { ok: false as const, messages };
      }
      return { ok: true as const, scenario: parsed.value };
    }
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

    if (format === "visual") {
      if (nextFormat === "json") {
        setScenarioText(JSON.stringify(scenarioObj, null, 2));
      } else if (nextFormat === "yaml") {
        setScenarioText(stringifyYAML(scenarioObj));
      }
      setAPIError(null);
    } else {
      const currentParsed = parseScenario(scenarioText, format);
      if (!currentParsed.ok) {
        setAPIError(`Cannot switch mode: ${currentParsed.message}`);
        return;
      }
      setAPIError(null);
      setScenarioObj(currentParsed.value);

      if (nextFormat === "json") {
        setScenarioText(JSON.stringify(currentParsed.value, null, 2));
      } else if (nextFormat === "yaml") {
        setScenarioText(stringifyYAML(currentParsed.value));
      }
    }

    setFormat(nextFormat);
  }

  const addRequest = () => {
    const newRequests = [...(scenarioObj.requests || [])];
    const newIndex = newRequests.length;
    newRequests.push({
      name: `request_${newIndex + 1}`,
      method: "GET",
      path: "/",
      headers: {}
    });
    setExpandedRequests(prev => ({ ...prev, [newIndex]: true }));
    updateScenarioObj({ ...scenarioObj, requests: newRequests });
  };

  const removeRequest = (index: number) => {
    const newRequests = [...(scenarioObj.requests || [])];
    newRequests.splice(index, 1);
    updateScenarioObj({ ...scenarioObj, requests: newRequests });
  };

  const updateRequest = (index: number, updatedFields: Partial<any>) => {
    const newRequests = [...(scenarioObj.requests || [])];
    newRequests[index] = { ...newRequests[index] as any, ...updatedFields };
    updateScenarioObj({ ...scenarioObj, requests: newRequests });
  };

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
              <button className={format === "visual" ? "active" : ""} type="button" onClick={() => switchFormat("visual")}>
                <WandSparkles size={16} />
                <span>Visual</span>
              </button>
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

          {format === "visual" ? (
            <div className="visual-editor">
              <section className="form-section">
                <h3>General Info</h3>
                <div className="form-grid">
                  <div className="form-group">
                    <label htmlFor="sc-name">Scenario Name</label>
                    <input
                      id="sc-name"
                      type="text"
                      value={scenarioObj.name || ""}
                      onChange={(e) => updateScenarioObj({ ...scenarioObj, name: e.target.value })}
                      placeholder="e.g. checkout-idempotency"
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor="sc-desc">Description</label>
                    <input
                      id="sc-desc"
                      type="text"
                      value={scenarioObj.description || ""}
                      onChange={(e) => updateScenarioObj({ ...scenarioObj, description: e.target.value })}
                      placeholder="Description of the test case..."
                    />
                  </div>
                </div>
              </section>

              <section className="form-section">
                <h3>Target Configuration</h3>
                <div className="form-grid single-col">
                  <div className="form-group">
                    <label htmlFor="sc-target-url">Base URL</label>
                    <input
                      id="sc-target-url"
                      type="text"
                      value={scenarioObj.target?.base_url || ""}
                      onChange={(e) =>
                        updateScenarioObj({
                          ...scenarioObj,
                          target: { ...(scenarioObj.target || {}), base_url: e.target.value }
                        })
                      }
                      placeholder="e.g. http://localhost:9090"
                    />
                  </div>
                </div>
              </section>

              <section className="form-section">
                <h3>Traffic Profile</h3>
                <div className="form-grid">
                  <div className="form-group">
                    <label htmlFor="sc-traffic-type">Traffic Type</label>
                    <select
                      id="sc-traffic-type"
                      value={scenarioObj.traffic?.type || "load"}
                      onChange={(e) =>
                        updateScenarioObj({
                          ...scenarioObj,
                          traffic: { ...(scenarioObj.traffic || {}), type: e.target.value }
                        })
                      }
                    >
                      <option value="load">Load (Constant Rate)</option>
                      <option value="burst">Burst</option>
                      <option value="spike">Spike</option>
                      <option value="race">Race</option>
                      <option value="retry_storm">Retry Storm</option>
                    </select>
                  </div>
                  <div className="form-group">
                    <label htmlFor="sc-concurrency">Concurrency</label>
                    <input
                      id="sc-concurrency"
                      type="number"
                      min="1"
                      value={scenarioObj.traffic?.concurrency ?? 1}
                      onChange={(e) =>
                        updateScenarioObj({
                          ...scenarioObj,
                          traffic: {
                            ...(scenarioObj.traffic || {}),
                            concurrency: parseInt(e.target.value) || 1
                          }
                        })
                      }
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor="sc-iterations">Iterations</label>
                    <input
                      id="sc-iterations"
                      type="number"
                      min="1"
                      value={scenarioObj.traffic?.iterations ?? 1}
                      onChange={(e) =>
                        updateScenarioObj({
                          ...scenarioObj,
                          traffic: {
                            ...(scenarioObj.traffic || {}),
                            iterations: parseInt(e.target.value) || 1
                          }
                        })
                      }
                    />
                  </div>
                  {scenarioObj.traffic?.type === "load" && (
                    <div className="form-group">
                      <label htmlFor="sc-rate-limit">Rate per Second (Optional)</label>
                      <input
                        id="sc-rate-limit"
                        type="number"
                        min="0"
                        value={scenarioObj.traffic?.rate_per_second ?? ""}
                        onChange={(e) =>
                          updateScenarioObj({
                            ...scenarioObj,
                            traffic: {
                              ...(scenarioObj.traffic || {}),
                              rate_per_second: e.target.value ? parseInt(e.target.value) : undefined
                            }
                          })
                        }
                        placeholder="e.g. 100"
                      />
                    </div>
                  )}
                </div>
              </section>

              <section className="form-section builder-section">
                <div className="section-header-row">
                  <h3>HTTP Requests</h3>
                  <button type="button" className="add-btn" onClick={addRequest}>
                    <Plus size={14} />
                    <span>Add Request</span>
                  </button>
                </div>

                <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
                  {((scenarioObj.requests || []) as any[]).map((req, index) => {
                    const isExpanded = expandedRequests[index] ?? false;
                    const headerPairs = Object.entries(req.headers || {});

                    return (
                      <div className="request-card" key={index}>
                        <div
                          className="request-card-header"
                          onClick={() => setExpandedRequests(prev => ({ ...prev, [index]: !isExpanded }))}
                        >
                          <div className="request-title-area">
                            <span className={`method-badge ${(req.method || "GET").toLowerCase()}`}>
                              {req.method || "GET"}
                            </span>
                            <span className="request-name-display">{req.name || `request_${index + 1}`}</span>
                            <span className="request-path-display">{req.path || "/"}</span>
                          </div>
                          <div className="header-actions">
                            <button
                              type="button"
                              className="delete-card-btn"
                              onClick={(e) => {
                                e.stopPropagation();
                                removeRequest(index);
                              }}
                              title="Delete Request"
                            >
                              <Trash2 size={16} />
                            </button>
                            {isExpanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                          </div>
                        </div>

                        {isExpanded && (
                          <div className="request-card-body">
                            <div className="form-grid">
                              <div className="form-group">
                                <label>Request Name</label>
                                <input
                                  type="text"
                                  value={req.name || ""}
                                  onChange={(e) => updateRequest(index, { name: e.target.value })}
                                  placeholder="e.g. get_user"
                                />
                              </div>
                              <div className="form-group">
                                <label>Method</label>
                                <select
                                  value={req.method || "GET"}
                                  onChange={(e) => updateRequest(index, { method: e.target.value })}
                                >
                                  <option value="GET">GET</option>
                                  <option value="POST">POST</option>
                                  <option value="PUT">PUT</option>
                                  <option value="DELETE">DELETE</option>
                                  <option value="PATCH">PATCH</option>
                                </select>
                              </div>
                              <div className="form-group span-2">
                                <label>Path</label>
                                <input
                                  type="text"
                                  value={req.path || ""}
                                  onChange={(e) => updateRequest(index, { path: e.target.value })}
                                  placeholder="e.g. /api/users"
                                />
                              </div>
                            </div>

                            <div className="headers-builder">
                              <div className="section-header-row" style={{ marginBottom: "4px" }}>
                                <span className="label" style={{ fontSize: "0.72rem" }}>Headers</span>
                                <button
                                  type="button"
                                  className="add-btn"
                                  style={{ height: "26px", fontSize: "0.75rem" }}
                                  onClick={() => {
                                    const newHeaders = { ...(req.headers || {}), "": "" };
                                    updateRequest(index, { headers: newHeaders });
                                  }}
                                >
                                  <Plus size={12} />
                                  <span>Add Header</span>
                                </button>
                              </div>

                              {headerPairs.map(([key, val], hIdx) => (
                                <div className="header-row" key={hIdx}>
                                  <input
                                    type="text"
                                    placeholder="Header Name"
                                    value={key}
                                    onChange={(e) => {
                                      const nextKey = e.target.value;
                                      const newHeaders: Record<string, string> = {};
                                      headerPairs.forEach(([k, v], i) => {
                                        if (i === hIdx) {
                                          newHeaders[nextKey] = String(v);
                                        } else {
                                          newHeaders[k] = String(v);
                                        }
                                      });
                                      updateRequest(index, { headers: newHeaders });
                                    }}
                                  />
                                  <input
                                    type="text"
                                    placeholder="Value"
                                    value={String(val)}
                                    onChange={(e) => {
                                      const nextVal = e.target.value;
                                      const newHeaders = { ...(req.headers || {}) };
                                      newHeaders[key] = nextVal;
                                      updateRequest(index, { headers: newHeaders });
                                    }}
                                  />
                                  <button
                                    type="button"
                                    className="delete-card-btn"
                                    onClick={() => {
                                      const newHeaders = { ...(req.headers || {}) };
                                      delete newHeaders[key];
                                      updateRequest(index, { headers: newHeaders });
                                    }}
                                  >
                                    <Trash2 size={14} />
                                  </button>
                                </div>
                              ))}
                            </div>

                            {["POST", "PUT", "PATCH", "DELETE"].includes(req.method || "GET") && (
                              <div className="form-group">
                                <label>JSON Body</label>
                                <textarea
                                  style={{ minHeight: "100px", fontFamily: "monospace" }}
                                  placeholder='e.g. { "id": 123 }'
                                  value={req.json ? JSON.stringify(req.json, null, 2) : req.body || ""}
                                  onChange={(e) => {
                                    const text = e.target.value;
                                    try {
                                      if (text.trim() === "") {
                                        updateRequest(index, { json: undefined, body: undefined });
                                      } else {
                                        const parsedJson = JSON.parse(text);
                                        updateRequest(index, { json: parsedJson, body: undefined });
                                      }
                                    } catch {
                                      updateRequest(index, { json: undefined, body: text });
                                    }
                                  }}
                                />
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </section>
            </div>
          ) : (
            <textarea
              spellCheck={false}
              value={scenarioText}
              onChange={(event) => {
                setScenarioText(event.target.value);
                setDirty(true);
              }}
              aria-label="Scenario definition"
            />
          )}
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

function parseScenario(text: string, format: "json" | "yaml"): { ok: true; value: ScenarioDefinition } | { ok: false; message: string } {
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
