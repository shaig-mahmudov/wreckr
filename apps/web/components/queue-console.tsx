"use client";

import { Activity, AlertTriangle, CheckCircle2, RefreshCw, Trash2 } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { useAPIURL } from "../lib/use-api-url";
import { fetchDeadletterTasks, retryTask, deleteTask, type TaskInfo } from "../lib/queue-api";

export function QueueConsole() {
  const [apiURL] = useAPIURL();
  const [tasks, setTasks] = useState<TaskInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadTasks = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await fetchDeadletterTasks(apiURL);
      setTasks(data || []);
    } catch (err: any) {
      setError(err.message || "Failed to load dead-letter queue");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadTasks();
  }, [apiURL]);

  const handleRetry = async (taskId: string) => {
    try {
      await retryTask(taskId, apiURL);
      await loadTasks();
    } catch (err: any) {
      alert(`Failed to retry task: ${err.message}`);
    }
  };

  const handleDelete = async (taskId: string) => {
    if (!confirm("Are you sure you want to delete this task?")) return;
    try {
      await deleteTask(taskId, apiURL);
      await loadTasks();
    } catch (err: any) {
      alert(`Failed to delete task: ${err.message}`);
    }
  };

  return (
    <div className="layout">
      <div className="sidebar">
        <div className="sidebar-header">
          <Link href="/" className="logo">
            <Activity className="logo-icon" size={24} />
            <span className="logo-text">Wreckr</span>
          </Link>
        </div>
      </div>

      <main className="main-content">
        <div className="topbar">
          <div>
            <p className="eyebrow">Wreckr</p>
            <h1>Queue Visibility Dashboard</h1>
          </div>
          <div className="topbar-actions">
            <nav className="nav-actions" aria-label="Dashboard navigation">
              <Link href="/">Console</Link>
              <Link href="/scenarios">Scenarios</Link>
              <Link href="/queues" className="active">Queues</Link>
            </nav>
          </div>
        </div>

        <div className="scroll-content">
          <div className="content-container">
            <section className="section">
              <div className="section-header">
                <h2>Dead-Letter Queue (Archived Tasks)</h2>
                <button className="btn-secondary" onClick={loadTasks} disabled={loading}>
                  <RefreshCw size={14} className={loading ? "spin" : ""} /> Refresh
                </button>
              </div>

              {error && (
                <div className="panel error-panel">
                  <AlertTriangle size={20} />
                  <div>
                    <h3 className="panel-title">Failed to load tasks</h3>
                    <p>{error}</p>
                  </div>
                </div>
              )}

              {!error && tasks.length === 0 && !loading && (
                <div className="empty-state">
                  <CheckCircle2 size={48} className="empty-icon" />
                  <h3>No dead-letter tasks</h3>
                  <p>The queue is healthy.</p>
                </div>
              )}

              {!error && tasks.length > 0 && (
                <div className="scenario-list">
                  {tasks.map(task => {
                    let runId = "Unknown";
                    try {
                      const payload = JSON.parse(task.payload);
                      runId = payload.run_id || "Unknown";
                    } catch (e) {}

                    return (
                      <div key={task.id} className="scenario-card">
                        <div className="scenario-card-header">
                          <div className="scenario-card-title">
                            <h3>Run ID: {runId}</h3>
                            <span className="version-badge">Retried: {task.retried}/{task.max_retry}</span>
                          </div>
                          <div className="scenario-card-actions">
                            <button className="btn-secondary" onClick={() => handleRetry(task.id)}>
                              <RefreshCw size={14} /> Retry
                            </button>
                            <button className="btn-danger" onClick={() => handleDelete(task.id)}>
                              <Trash2 size={14} /> Delete
                            </button>
                          </div>
                        </div>
                        <div className="scenario-card-meta">
                          <span>Task ID: {task.id}</span>
                          <span>Failed At: {new Date(task.last_failed_at).toLocaleString()}</span>
                        </div>
                        <p className="scenario-card-desc error-text">
                          {task.last_err}
                        </p>
                      </div>
                    );
                  })}
                </div>
              )}
            </section>
          </div>
        </div>
      </main>
    </div>
  );
}
