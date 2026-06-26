export type ScenarioDefinition = {
  version?: number;
  name?: string;
  description?: string;
  target?: {
    base_url?: string;
    headers?: Record<string, string>;
  };
  traffic?: {
    type?: string;
    concurrency?: number;
    iterations?: number;
    rate_per_second?: number;
    retry?: {
      attempts?: number;
      backoff_ms?: number;
    };
  };
  setup?: unknown[];
  requests?: unknown[];
  teardown?: unknown[];
  thresholds?: Record<string, unknown>;
  invariants?: unknown[];
  [key: string]: unknown;
};

export type ScenarioRecord = {
  id: string;
  scenario: ScenarioDefinition;
  current_version_id?: string;
  current_version_number?: number;
  created_at: string;
};

export type ScenarioVersionRecord = {
  id: string;
  scenario_id: string;
  version_number: number;
  scenario: ScenarioDefinition;
  created_at: string;
};

export type RunRecord = {
  id: string;
  scenario_id?: string;
  scenario_version_id?: string;
  scenario_version_number?: number;
  status: "queued" | "running" | "passed" | "failed" | "errored" | "canceled";
  scenario: ScenarioDefinition;
  created_at?: string;
  error?: string;
};

type APIErrorPayload = {
  error?: string;
};

export const defaultAPIURL = process.env.NEXT_PUBLIC_WRECKR_API_URL ?? "http://localhost:8080";

export async function apiRequest<T>(apiURL: string, path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${apiURL.replace(/\/$/, "")}${path}`, {
    cache: "no-store",
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    }
  });
  const payload = (await response.json().catch(() => ({}))) as T & APIErrorPayload;
  if (!response.ok) {
    throw new Error(payload.error ?? `Request failed with ${response.status}`);
  }
  return payload;
}

export function sortScenarios(records: ScenarioRecord[]) {
  return [...records].sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at));
}

export function sortVersions(records: ScenarioVersionRecord[]) {
  return [...records].sort((a, b) => b.version_number - a.version_number);
}
