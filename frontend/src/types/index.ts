export interface AlertRecord {
  status: "firing" | "resolved";
  fingerprint: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  startsAt: string;
  endsAt: string;
  generatorURL?: string;
}

export interface ApiResponse<T> {
  status: string;
  data?: T;
  error?: string;
}
