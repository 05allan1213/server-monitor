export interface AlertRecord {
  status: string;
  fingerprint: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  startsAt: string;
  endsAt: string;
  generatorURL?: string;
}

export interface ApiResponse<T> {
  status: string;
  data: T;
  error?: string;
}
