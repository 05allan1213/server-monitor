import { getApiData } from "./client";
import type { Host } from "../types";

export interface HostsQuery {
  status?: "all" | "up" | "down";
  q?: string;
  sort?: "instance" | "cpu_desc" | "memory_desc";
  risk?: "all" | "high_cpu" | "high_memory";
}

export async function fetchHosts(query: HostsQuery = {}): Promise<Host[]> {
  const params: Record<string, string> = {};

  if (query.status && query.status !== "all") {
    params.status = query.status;
  }
  if (query.q) {
    params.q = query.q;
  }
  if (query.sort && query.sort !== "instance") {
    params.sort = query.sort;
  }
  if (query.risk && query.risk !== "all") {
    params.risk = query.risk;
  }

  return (await getApiData<Host[]>("/api/v1/hosts", { params })) ?? [];
}
