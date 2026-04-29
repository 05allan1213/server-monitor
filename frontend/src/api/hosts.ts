import type { Host, ApiResponse } from "../types";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

export interface HostsQuery {
  status?: "all" | "up" | "down";
  q?: string;
  sort?: "instance" | "cpu_desc" | "memory_desc";
}

export async function fetchHosts(query: HostsQuery = {}): Promise<Host[]> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 10000);

  try {
    const url = new URL(`${apiBaseUrl}/api/v1/hosts`, window.location.origin);
    if (query.status && query.status !== "all") {
      url.searchParams.set("status", query.status);
    }
    if (query.q) {
      url.searchParams.set("q", query.q);
    }
    if (query.sort && query.sort !== "instance") {
      url.searchParams.set("sort", query.sort);
    }

    const response = await fetch(url.toString(), {
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
