import { apiRequest, defaultAPIURL } from "./scenario-api";

export type TaskInfo = {
  id: string;
  queue: string;
  type: string;
  state: string;
  max_retry: number;
  retried: number;
  last_err: string;
  last_failed_at: string;
  payload: string;
};

export async function fetchDeadletterTasks(apiURL = defaultAPIURL): Promise<TaskInfo[]> {
  return apiRequest<TaskInfo[]>(apiURL, "/v1/queue/deadletter");
}

export async function retryTask(taskId: string, apiURL = defaultAPIURL): Promise<void> {
  return apiRequest<void>(apiURL, `/v1/queue/tasks/${encodeURIComponent(taskId)}/retry`, {
    method: "POST"
  });
}

export async function deleteTask(taskId: string, apiURL = defaultAPIURL): Promise<void> {
  return apiRequest<void>(apiURL, `/v1/queue/tasks/${encodeURIComponent(taskId)}`, {
    method: "DELETE"
  });
}
