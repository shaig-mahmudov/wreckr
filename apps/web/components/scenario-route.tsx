"use client";

import { AlertTriangle, RefreshCw } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { apiRequest, ScenarioRecord, ScenarioVersionRecord } from "../lib/scenario-api";
import { useAPIURL } from "../lib/use-api-url";
import { ScenarioEditor } from "./scenario-editor";

type VersionsResponse = {
  versions?: ScenarioVersionRecord[];
};

type ScenarioRouteProps = {
  scenarioID: string;
  versionNumber?: number;
};

export function ScenarioRoute({ scenarioID, versionNumber }: ScenarioRouteProps) {
  const [apiURL, setAPIURL] = useAPIURL();
  const [scenario, setScenario] = useState<ScenarioRecord | null>(null);
  const [version, setVersion] = useState<ScenarioVersionRecord | null>(null);
  const [versions, setVersions] = useState<ScenarioVersionRecord[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const selectedScenario = version?.scenario ?? scenario?.scenario;
  const selectedVersionNumber = version?.version_number ?? scenario?.current_version_number;
  const pageTitle = useMemo(() => {
    if (versionNumber) {
      return `${selectedScenario?.name ?? "Scenario"} v${versionNumber}`;
    }
    return selectedScenario?.name ?? "Edit Scenario";
  }, [selectedScenario?.name, versionNumber]);

  useEffect(() => {
    void loadScenario();
  }, [apiURL, scenarioID, versionNumber]);

  async function loadScenario() {
    setLoading(true);
    setError(null);
    try {
      const [record, versionList] = await Promise.all([
        apiRequest<ScenarioRecord>(apiURL, `/v1/scenarios/${scenarioID}`),
        apiRequest<VersionsResponse>(apiURL, `/v1/scenarios/${scenarioID}/versions`)
      ]);
      setScenario(record);
      setVersions(versionList.versions ?? []);
      if (versionNumber) {
        const selected = await apiRequest<ScenarioVersionRecord>(apiURL, `/v1/scenarios/${scenarioID}/versions/${versionNumber}`);
        setVersion(selected);
      } else {
        setVersion(null);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load scenario.");
      setScenario(null);
      setVersion(null);
      setVersions([]);
    } finally {
      setLoading(false);
    }
  }

  if (loading || error || !selectedScenario) {
    return (
      <main className="app-shell">
        <header className="topbar">
          <div>
            <p className="eyebrow">Scenario</p>
            <h1>{loading ? "Loading Scenario" : "Scenario Unavailable"}</h1>
          </div>
          <nav className="nav-actions" aria-label="Dashboard navigation">
            <Link href="/">Console</Link>
            <Link href="/scenarios">Scenarios</Link>
          </nav>
        </header>
        <section className="list-toolbar">
          <label>
            <span className="label">API</span>
            <input value={apiURL} onChange={(event) => setAPIURL(event.target.value)} />
          </label>
          <button className="icon-button" type="button" onClick={loadScenario} title="Reload scenario">
            <RefreshCw size={18} />
          </button>
        </section>
        {error ? (
          <div className="notice bad">
            <AlertTriangle size={18} />
            <span>{error}</span>
          </div>
        ) : (
          <p className="empty-state">Loading scenario definition</p>
        )}
      </main>
    );
  }

  return (
    <ScenarioEditor
      key={`${scenarioID}-${selectedVersionNumber}`}
      eyebrow={versionNumber ? "Scenario Version" : "Current Scenario"}
      initialScenario={selectedScenario}
      saveLabel="Save Version"
      scenarioID={scenarioID}
      selectedVersionNumber={selectedVersionNumber}
      title={pageTitle}
      versions={versions}
    />
  );
}
