import type { AlertRecord, ApiResponse } from "../types";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

export async function fetchActiveAlerts(): Promise<AlertRecord[]> {
  const response = await fetch(`${apiBaseUrl}/api/v1/alerts/active`);

  if (!response.ok) {
    throw new Error(`Request failed with status ${response.status}`);
  }

  const payload = (await response.json()) as ApiResponse<AlertRecord[]>;
  if (payload.status !== "success") {
    throw new Error(payload.error ?? "Unknown API error");
  }

  return payload.data;
}
