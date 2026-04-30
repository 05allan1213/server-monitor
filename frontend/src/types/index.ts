export interface AlertRecord {
  status: "firing" | "resolved";
  fingerprint: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  startsAt: string;
  endsAt: string;
  generatorURL?: string;
}

export interface AlertEvent {
  status: "firing" | "resolved";
  fingerprint: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  startsAt: string;
  endsAt: string;
  generatorURL?: string;
  receivedAt: string;
}

export interface Host {
  instance: string;
  cpu: number;
  memory: number;
  status: string;
  lastScrape: string;
}

export interface RangePoint {
  timestamp: string;
  value: number;
}

export interface RangeSeries {
  metric: Record<string, string>;
  values: RangePoint[];
}

export interface HostMetricsResponse {
  instance: string;
  range: string;
  stepSeconds: number;
  metrics: Record<string, RangeSeries[]>;
}

export interface DashboardOverview {
  total_hosts: number;
  healthy_hosts: number;
  down_hosts: number;
  active_alerts: number;
  avg_cpu: number;
  avg_memory: number;
  generated_at: string;
  alert_degraded?: boolean;
}

export interface DependencyStatus {
  prometheus?: string;
  redis?: string;
}

export interface HealthStatus {
  healthy?: boolean;
}

export interface ReadyStatus {
  ready?: boolean;
  dependencies?: DependencyStatus;
}

export interface ApiResponse<T> {
  status: string;
  data?: T;
  error?: string;
}
