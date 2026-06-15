"use client";

import { Activity, AlertTriangle, Plus, RefreshCw, Server, Trash2, Edit2 } from "lucide-react";
import { useEffect, useState } from "react";

export type TargetEnvironment = "local" | "development" | "staging" | "production";

export type TargetRecord = {
  id: string;
  name: string;
  base_url: string;
  environment: TargetEnvironment;
  description?: string;
  headers?: Record<string, string>;
  created_at?: string;
  updated_at?: string;
};

type TargetsResponse = {
  targets?: TargetRecord[];
  error?: string;
};

export function TargetManager({ apiURL }: { apiURL: string }) {
  const [targets, setTargets] = useState<TargetRecord[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [apiState, setAPIState] = useState<"checking" | "connected" | "offline">("checking");
  const [isEditing, setIsEditing] = useState<boolean>(false);
  
  // Form state
  const [editId, setEditId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [environment, setEnvironment] = useState<TargetEnvironment>("development");
  const [description, setDescription] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void refreshTargets();
  }, [apiURL]);

  async function refreshTargets() {
    setAPIState("checking");
    setError(null);
    try {
      const baseURL = apiURL.replace(/\/$/, "");
      const response = await fetch(`${baseURL}/v1/targets`, { cache: "no-store" });
      const payload = (await response.json()) as TargetsResponse;
      if (!response.ok) {
        throw new Error(payload.error ?? `Target list failed with ${response.status}`);
      }
      setTargets(payload.targets ?? []);
      setAPIState("connected");
    } catch (err) {
      setAPIState("offline");
      setTargets([]);
      setError(err instanceof Error ? err.message : "Could not connect to the Wreckr API.");
    }
  }

  function startEdit(target?: TargetRecord) {
    if (target) {
      setEditId(target.id);
      setName(target.name);
      setBaseUrl(target.base_url);
      setEnvironment(target.environment);
      setDescription(target.description || "");
    } else {
      setEditId(null);
      setName("");
      setBaseUrl("");
      setEnvironment("development");
      setDescription("");
    }
    setIsEditing(true);
    setError(null);
  }

  function cancelEdit() {
    setIsEditing(false);
    setError(null);
  }

  async function saveTarget(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const baseURL = apiURL.replace(/\/$/, "");
      const method = editId ? "PUT" : "POST";
      const url = editId ? `${baseURL}/v1/targets/${editId}` : `${baseURL}/v1/targets`;
      
      const payload = {
        name,
        baseUrl: baseUrl,
        environment,
        description,
      };

      const response = await fetch(url, {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error ?? `Save failed with ${response.status}`);
      }
      
      setIsEditing(false);
      await refreshTargets();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed.");
    } finally {
      setBusy(false);
    }
  }

  async function deleteTarget(id: string) {
    if (!confirm("Are you sure you want to delete this target?")) return;
    setBusy(true);
    setError(null);
    try {
      const baseURL = apiURL.replace(/\/$/, "");
      const response = await fetch(`${baseURL}/v1/targets/${id}`, {
        method: "DELETE",
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error ?? `Delete failed with ${response.status}`);
      }
      
      await refreshTargets();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Delete failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="scenario-panel" aria-label="Target Manager">
      <div className="panel-head">
        <div>
          <span className="label">Management</span>
          <h2>Targets</h2>
        </div>
        {!isEditing && (
          <button className="run-button" style={{minWidth: 'auto', gap: 6}} type="button" onClick={() => startEdit()}>
            <Plus size={16} />
            <span>New Target</span>
          </button>
        )}
      </div>

      {error && (
        <div className="notice bad" style={{marginBottom: 16}}>
          <AlertTriangle size={18} />
          <span>{error}</span>
        </div>
      )}

      {isEditing ? (
        <form onSubmit={saveTarget} className="run-list" style={{padding: 20, display: 'flex', flexDirection: 'column', gap: 16}}>
          <div className="section-title" style={{padding: 0, minHeight: 'auto', border: 0, paddingBottom: 10, marginBottom: 10, borderBottom: '1px solid var(--line)'}}>
            <Server size={17} />
            <h2>{editId ? "Edit Target" : "New Target"}</h2>
          </div>
          
          <div style={{display: 'flex', flexDirection: 'column', gap: 6}}>
            <label className="label">Name</label>
            <input 
              required
              value={name} 
              onChange={e => setName(e.target.value)} 
              className="toolbar input" 
              style={{height: 40, padding: '0 12px', border: '1px solid var(--line)', borderRadius: 8}}
              placeholder="E.g. Production Search API" 
            />
          </div>

          <div style={{display: 'flex', flexDirection: 'column', gap: 6}}>
            <label className="label">Base URL</label>
            <input 
              required
              type="url"
              value={baseUrl} 
              onChange={e => setBaseUrl(e.target.value)} 
              style={{height: 40, padding: '0 12px', border: '1px solid var(--line)', borderRadius: 8}}
              placeholder="https://api.example.com" 
            />
          </div>

          <div style={{display: 'flex', flexDirection: 'column', gap: 6}}>
            <label className="label">Environment</label>
            <select 
              value={environment} 
              onChange={e => setEnvironment(e.target.value as TargetEnvironment)}
              style={{height: 40, padding: '0 12px', border: '1px solid var(--line)', borderRadius: 8, background: 'var(--surface)'}}
            >
              <option value="local">Local</option>
              <option value="development">Development</option>
              <option value="staging">Staging</option>
              <option value="production">Production</option>
            </select>
          </div>

          <div style={{display: 'flex', flexDirection: 'column', gap: 6}}>
            <label className="label">Description</label>
            <textarea 
              value={description} 
              onChange={e => setDescription(e.target.value)} 
              style={{minHeight: 80, height: 80, resize: 'none', background: 'var(--surface)', color: 'var(--ink)'}}
              placeholder="Optional details about this target" 
            />
          </div>

          <div style={{display: 'flex', gap: 12, marginTop: 10}}>
            <button type="submit" disabled={busy} className="run-button" style={{flex: 1}}>
              {busy ? "Saving..." : "Save Target"}
            </button>
            <button type="button" disabled={busy} onClick={cancelEdit} className="text-button" style={{border: '1px solid var(--line)', height: 40, padding: '0 16px', color: 'var(--ink)'}}>
              Cancel
            </button>
          </div>
        </form>
      ) : (
        <section className="run-list" aria-label="Target list">
          <div className="section-title">
            <Server size={17} />
            <h2>Available Targets</h2>
            <button className="text-button" type="button" onClick={refreshTargets} disabled={busy}>
              <RefreshCw size={14} style={{marginRight: 4}}/>
              Refresh
            </button>
          </div>
          {targets.length > 0 ? (
            <div className="run-items">
              {targets.map((target) => (
                <div key={target.id} className="run-item" style={{display: 'flex', alignItems: 'center'}}>
                  <span style={{flex: 1}}>
                    <strong>{target.name}</strong>
                    <small>{target.base_url}</small>
                  </span>
                  <em className={target.environment === 'production' ? 'bad' : target.environment === 'staging' ? 'running' : 'passed'} style={{marginRight: 10}}>
                    {target.environment}
                  </em>
                  <div style={{display: 'flex', gap: 8}}>
                    <button className="icon-button" style={{width: 32, height: 32}} onClick={() => startEdit(target)} title="Edit">
                      <Edit2 size={14} />
                    </button>
                    <button className="icon-button" style={{width: 32, height: 32, color: 'var(--red)'}} onClick={() => deleteTarget(target.id)} title="Delete">
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="empty-state">{apiState === "connected" ? "No targets configured yet" : "Connect to the API to load targets"}</p>
          )}
        </section>
      )}
    </section>
  );
}
