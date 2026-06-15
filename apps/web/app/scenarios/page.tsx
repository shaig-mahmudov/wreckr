"use client";

import { AlertTriangle, FilePlus2, RefreshCw, Server } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { apiRequest, ScenarioRecord, sortScenarios } from "../../lib/scenario-api";
import { useAPIURL } from "../../lib/use-api-url";

type ScenariosResponse = {
  scenarios?: ScenarioRecord[];
};

export default function ScenariosPage() {
  const [apiURL, setAPIURL] = useAPIURL();
  const [scenarios, setScenarios] = useState<ScenarioRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void loadScenarios();
  }, [apiURL]);

  async function loadScenarios() {
    setLoading(true);
    setError(null);
    try {
      const payload = await apiRequest<ScenariosResponse>(apiURL, "/v1/scenarios");
      setScenarios(sortScenarios(payload.scenarios ?? []));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load scenarios.");
      setScenarios([]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Scenarios</p>
          <h1>Scenario Library</h1>
        </div>
        <nav className="nav-actions" aria-label="Dashboard navigation">
          <Link href="/">Console</Link>
          <Link href="/scenarios/new">New Scenario</Link>
        </nav>
      </header>

      <section className="list-toolbar">
        <label>
          <span className="label">API</span>
          <input value={apiURL} onChange={(event) => setAPIURL(event.target.value)} />
        </label>
        <button className="icon-button" type="button" onClick={loadScenarios} title="Refresh scenarios">
          <RefreshCw size={18} />
        </button>
        <Link className="run-button" href="/scenarios/new">
          <FilePlus2 size={18} />
          <span>Create</span>
        </Link>
      </section>

      {error ? (
        <div className="notice bad">
          <AlertTriangle size={18} />
          <span>{error}</span>
        </div>
      ) : null}

      <section className="scenario-list" aria-label="Saved scenarios">
        {loading ? <p className="empty-state">Loading scenarios</p> : null}
        {!loading && scenarios.length === 0 ? <p className="empty-state">No scenarios have been saved yet</p> : null}
        {scenarios.map((record) => (
          <Link key={record.id} className="scenario-card" href={`/scenarios/${record.id}`}>
            <div>
              <strong>{record.scenario.name ?? "Untitled scenario"}</strong>
              <span>{record.scenario.description ?? record.id}</span>
            </div>
            <dl>
              <div>
                <dt>Version</dt>
                <dd>v{record.current_version_number ?? 1}</dd>
              </div>
              <div>
                <dt>Target</dt>
                <dd>{record.scenario.target?.base_url ?? "-"}</dd>
              </div>
              <div>
                <dt>Created</dt>
                <dd>{new Date(record.created_at).toLocaleDateString()}</dd>
              </div>
            </dl>
            <Server size={18} />
          </Link>
        ))}
      </section>
    </main>
  );
}
