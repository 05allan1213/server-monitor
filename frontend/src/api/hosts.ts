import type { Host, ApiResponse } from "../types";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

export async function fetchHosts(): Promise<Host[]> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 10000);

  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/hosts`, {
      signal: controller.signal,
    });

    if (!response.ok) {
      throw new Error(`Request failed with status ${response.status}`);
    }

    const payload = (await response.json()) as ApiResponse<Host[]>;
    if (payload.status !== "success") {
      throw new Error(payload.error ?? "Unknown API error");
    }

    return payload.data ?? [];
  } finally {
    clearTimeout(timeoutId);
  }
}
