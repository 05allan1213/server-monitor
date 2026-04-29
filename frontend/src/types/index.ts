export interface AlertRecord {
  status: "firing" | "resolved";
  fingerprint: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  startsAt: string;
  endsAt: string;
  generatorURL?: string;
}

export interface Host {
  instance: string;
  cpu: number;
  memory: number;
  status: string;
  lastScrape: string;
}

export interface ApiResponse<T> {
  status: string;
  data?: T;
  error?: string;
}
