import type { AlertEvent, AlertRecord, ApiResponse } from "../types";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

export interface AlertEventsQuery {
  limit?: number;
  status?: "firing" | "resolved" | "all";
  severity?: "critical" | "warning" | "info" | "all";
}

export async function fetchActiveAlerts(): Promise<AlertRecord[]> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 10000);

  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/alerts/active`, {
      signal: controller.signal,
    });

    if (!response.ok) {
      throw new Error(`Request failed with status ${response.status}`);
    }

    const payload = (await response.json()) as ApiResponse<AlertRecord[]>;
    if (payload.status !== "success") {
      throw new Error(payload.error ?? "Unknown API error");
    }

    return payload.data ?? [];
  } finally {
    clearTimeout(timeoutId);
  }
}

export async function fetchAlertEvents(
  queryInput: AlertEventsQuery = {},
): Promise<AlertEvent[]> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 10000);
  const query = new URLSearchParams();

  query.set("limit", String(queryInput.limit ?? 8));
  if (queryInput.status && queryInput.status !== "all") {
    query.set("status", queryInput.status);
  }
  if (queryInput.severity && queryInput.severity !== "all") {
    query.set("severity", queryInput.severity);
  }

  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/alerts/events?${query.toString()}`, {
      signal: controller.signal,
    });

    if (!response.ok) {
      throw new Error(`Request failed with status ${response.status}`);
    }

    const payload = (await response.json()) as ApiResponse<AlertEvent[]>;
    if (payload.status !== "success") {
      throw new Error(payload.error ?? "Unknown API error");
    }

    return payload.data ?? [];
  } finally {
    clearTimeout(timeoutId);
  }
}
