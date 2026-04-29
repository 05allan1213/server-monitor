import type { AlertEvent, AlertRecord, ApiResponse } from "../types";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

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

export async function fetchAlertEvents(limit = 8): Promise<AlertEvent[]> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 10000);
  const query = new URLSearchParams({ limit: String(limit) });

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
