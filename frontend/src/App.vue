<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from "vue";

import { fetchActiveAlerts, fetchAlertEvents } from "./api/alerts";
import { fetchHosts } from "./api/hosts";
import { useAlertsWebSocket } from "./composables/useAlertsWebSocket";
import type { AlertEvent, AlertRecord, Host } from "./types";

const alerts = ref<AlertRecord[]>([]);
const alertEvents = ref<AlertEvent[]>([]);
const hosts = ref<Host[]>([]);
const loading = ref(true);
const refreshing = ref(false);
const alertsError = ref("");
const alertEventsError = ref("");
const hostSearchInput = ref("");
const appliedHostQuery = ref("");
const selectedSeverity = ref<"all" | "critical" | "warning" | "info">("all");
const beijingTime = ref("");
const beijingTimer = ref<number | null>(null);
const lastUpdateTime = ref(Date.now());
const updateAgo = ref("");
const updateAgoTimer = ref<number | null>(null);
const isFullscreen = ref(false);
const toasts = ref<{ id: number; message: string; severity: string }[]>(
  [],
);
let toastId = 0;
const toastTimers: number[] = [];

const { connectionState, connect, disconnect } = useAlertsWebSocket(
  applyIncomingAlert,
  applyIncomingHosts,
);

const criticalCount = computed(
  () =>
    alerts.value.filter((a) => (a.labels.severity ?? "info") === "critical")
      .length,
);
const warningCount = computed(
  () =>
    alerts.value.filter((a) => (a.labels.severity ?? "info") === "warning")
      .length,
);
const infoCount = computed(
  () =>
    alerts.value.filter((a) => (a.labels.severity ?? "info") === "info")
      .length,
);
const alertEventsLimit = 8;
const selectedHostStatus = ref<"all" | "up" | "down">("all");
const selectedHostSort = ref<"instance" | "cpu_desc" | "memory_desc">("instance");
const selectedEventStatus = ref<"all" | "firing" | "resolved">("all");
const selectedEventSeverity = ref<"all" | "critical" | "warning" | "info">("all");

const latestAlertEvents = computed(() => alertEvents.value.slice(0, alertEventsLimit));
const hostCountLabel = computed(() => {
  switch (selectedHostStatus.value) {
    case "up":
      return "在线主机";
    case "down":
      return "离线主机";
    default:
      return "当前主机";
  }
});

const connectionLabel = computed(() => {
  switch (connectionState.value) {
    case "connected":
      return "实时连接";
    case "connecting":
      return "连接中";
    case "disconnected":
      return "离线";
  }
});

watch(
  () => alerts.value.length,
  (newLen, oldLen) => {
    if (newLen > oldLen) {
      document.title =
        newLen > 0 ? `(${newLen}) 服务监控大屏` : "服务监控大屏";
    } else if (newLen === 0) {
      document.title = "服务监控大屏";
    }
  },
);

function severityClass(severity: string | undefined): string {
  switch (severity ?? "info") {
    case "critical":
      return "severity-critical";
    case "warning":
      return "severity-warning";
    default:
      return "severity-info";
  }
}

function severityLabel(severity: string | undefined): string {
  switch (severity ?? "info") {
    case "critical":
      return "严重";
    case "warning":
      return "警告";
    default:
      return "提示";
  }
}

function cpuColor(value: number): string {
  if (value > 80) return "var(--danger)";
  if (value > 60) return "var(--warning)";
  return "var(--success)";
}

function memoryColor(value: number): string {
  if (value > 85) return "var(--danger)";
  if (value > 70) return "var(--warning)";
  return "var(--success)";
}

function isHostUp(status: string): boolean {
  return status === "up" || status === "healthy";
}

function matchesHostQuery(host: Host, query: string): boolean {
  if (!query) {
    return true;
  }

  return host.instance.toLowerCase().includes(query);
}

function sortHosts(hostList: Host[]): Host[] {
  const sorted = [...hostList];

  switch (selectedHostSort.value) {
    case "cpu_desc":
      sorted.sort((a, b) => {
        if (b.cpu === a.cpu) {
          return a.instance.localeCompare(b.instance);
        }
        return b.cpu - a.cpu;
      });
      break;
    case "memory_desc":
      sorted.sort((a, b) => {
        if (b.memory === a.memory) {
          return a.instance.localeCompare(b.instance);
        }
        return b.memory - a.memory;
      });
      break;
    default:
      sorted.sort((a, b) => a.instance.localeCompare(b.instance));
  }

  return sorted;
}

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString("zh-CN");
  } catch {
    return iso;
  }
}

function setSeverityFilter(value: "all" | "critical" | "warning" | "info") {
  selectedSeverity.value = value;
  loadAlerts();
}

function setHostStatusFilter(value: "all" | "up" | "down") {
  selectedHostStatus.value = value;
  loadHosts();
}

function setHostSort(value: "instance" | "cpu_desc" | "memory_desc") {
  selectedHostSort.value = value;
  loadHosts();
}

function applyHostSearch() {
  appliedHostQuery.value = hostSearchInput.value.trim().toLowerCase();
  loadHosts();
}

function setEventStatusFilter(value: "all" | "firing" | "resolved") {
  selectedEventStatus.value = value;
  loadAlertEvents();
}

function setEventSeverityFilter(
  value: "all" | "critical" | "warning" | "info",
) {
  selectedEventSeverity.value = value;
  loadAlertEvents();
}

async function loadAlerts() {
  try {
    alertsError.value = "";
    const data = await fetchActiveAlerts({
      severity: selectedSeverity.value,
    });
    alerts.value = data;
  } catch (err) {
    alertsError.value = err instanceof Error ? err.message : "加载告警失败";
  }
}

async function loadAlertEvents() {
  try {
    alertEventsError.value = "";
    const data = await fetchAlertEvents({
      limit: alertEventsLimit,
      status: selectedEventStatus.value,
      severity: selectedEventSeverity.value,
    });
    alertEvents.value = data;
  } catch (err) {
    alertEventsError.value = err instanceof Error ? err.message : "加载事件失败";
  }
}

async function loadHosts() {
  try {
    const data = await fetchHosts({
      status: selectedHostStatus.value,
      q: appliedHostQuery.value,
      sort: selectedHostSort.value,
    });
    hosts.value = sortHosts(data);
  } catch (err) {
    console.error("loadHosts failed:", err);
  }
}

async function refreshAll() {
  refreshing.value = true;
  try {
    await Promise.all([loadAlerts(), loadAlertEvents(), loadHosts()]);
    lastUpdateTime.value = Date.now();
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
}

function pushAlertEvent(event: AlertEvent) {
  alertEvents.value.unshift(event);
  if (alertEvents.value.length > 200) {
    alertEvents.value.length = 200;
  }
}

function applyIncomingAlert(event: AlertEvent) {
  pushAlertEvent(event);

  const alert: AlertRecord = {
    status: event.status,
    fingerprint: event.fingerprint,
    labels: event.labels,
    annotations: event.annotations,
    startsAt: event.startsAt,
    endsAt: event.endsAt,
    generatorURL: event.generatorURL,
  };

  const idx = alerts.value.findIndex(
    (a) => a.fingerprint === alert.fingerprint,
  );
  if (alert.status === "resolved") {
    if (idx !== -1) alerts.value.splice(idx, 1);
    return;
  }
  if (idx !== -1) {
    alerts.value[idx] = alert;
  } else {
    alerts.value.unshift(alert);
    const sev = alert.labels.severity ?? "info";
    const sevLabel = severityLabel(sev);
    const name = alert.labels.alertname || "未知告警";
    showToast(`新${sevLabel}告警: ${name}`, sev);
  }
}

function applyIncomingHosts(newHosts: Host[]) {
  hosts.value = sortHosts(newHosts.filter((host) => {
    const statusMatched =
      selectedHostStatus.value === "all" ||
      isHostUp(host.status) === (selectedHostStatus.value === "up");

    return statusMatched && matchesHostQuery(host, appliedHostQuery.value);
  }));
  lastUpdateTime.value = Date.now();
}

function showToast(message: string, severity: string) {
  const id = ++toastId;
  toasts.value.push({ id, message, severity });
  if (toasts.value.length > 10) {
    toasts.value.splice(0, toasts.value.length - 10);
  }
  const timerId = window.setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id);
  }, 4000);
  toastTimers.push(timerId);
}

function eventStatusLabel(status: AlertEvent["status"]): string {
  return status === "resolved" ? "恢复" : "触发";
}

function eventStatusClass(status: AlertEvent["status"]): string {
  return status === "resolved" ? "event-status-resolved" : "event-status-firing";
}

function updateBeijingTime() {
  beijingTime.value = new Date().toLocaleString("zh-CN", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function updateAgoText() {
  const diff = Math.floor((Date.now() - lastUpdateTime.value) / 1000);
  if (diff < 5) {
    updateAgo.value = "刚刚更新";
  } else if (diff < 60) {
    updateAgo.value = `${diff}秒前更新`;
  } else if (diff < 3600) {
    updateAgo.value = `${Math.floor(diff / 60)}分钟前更新`;
  } else {
    updateAgo.value = `${Math.floor(diff / 3600)}小时前更新`;
  }
}

function toggleFullscreen() {
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen().catch(() => {});
  } else {
    document.exitFullscreen().catch(() => {});
  }
}

function onFullscreenChange() {
  isFullscreen.value = !!document.fullscreenElement;
}

function onKeydown(e: KeyboardEvent) {
  if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
    return;
  }
  if ((e.target as HTMLElement).isContentEditable) {
    return;
  }
  if (e.ctrlKey || e.altKey || e.metaKey) {
    return;
  }
  if (e.key === "r" || e.key === "R") {
    e.preventDefault();
    refreshAll();
  } else if (e.key === "f" || e.key === "F") {
    e.preventDefault();
    toggleFullscreen();
  }
}

onMounted(() => {
  refreshAll();
  connect();
  updateBeijingTime();
  updateAgoText();
  beijingTimer.value = window.setInterval(updateBeijingTime, 1000);
  updateAgoTimer.value = window.setInterval(updateAgoText, 5000);
  window.addEventListener("keydown", onKeydown);
  document.addEventListener("fullscreenchange", onFullscreenChange);
});

onBeforeUnmount(() => {
  disconnect();
  if (beijingTimer.value !== null) clearInterval(beijingTimer.value);
  if (updateAgoTimer.value !== null) clearInterval(updateAgoTimer.value);
  toastTimers.forEach((id) => clearTimeout(id));
  toastTimers.length = 0;
  window.removeEventListener("keydown", onKeydown);
  document.removeEventListener("fullscreenchange", onFullscreenChange);
});
</script>

<template>
  <div class="app-container">
    <!-- Toast Notifications -->
    <div class="toast-container">
      <transition-group name="toast">
        <div
          v-for="toast in toasts"
          :key="toast.id"
          class="toast"
          :class="severityClass(toast.severity)"
        >
          <span class="toast-severity">{{ severityLabel(toast.severity) }}</span>
          <span class="toast-message">{{ toast.message }}</span>
        </div>
      </transition-group>
    </div>

    <!-- Header -->
    <header class="header">
      <div class="header-left">
        <div class="logo">
          <div class="logo-icon"></div>
          <div class="logo-text">
            <h1>服务监控大屏</h1>
            <p class="logo-sub">实时主机指标与告警推送</p>
          </div>
        </div>
      </div>
      <div class="header-right">
        <div class="update-ago">{{ updateAgo }}</div>
        <div class="clock">
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="12" cy="12" r="10" />
            <polyline points="12 6 12 12 16 14" />
          </svg>
          <span>{{ beijingTime }}</span>
        </div>
        <button class="fullscreen-btn" title="全屏 (F)" @click="toggleFullscreen">
          <svg
            v-if="!isFullscreen"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3" />
          </svg>
          <svg
            v-else
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path d="M8 3v3a2 2 0 0 1-2 2H3m18 0h-3a2 2 0 0 1-2-2V3m0 18v-3a2 2 0 0 1 2-2h3M3 16h3a2 2 0 0 1 2 2v3" />
          </svg>
        </button>
        <div class="ws-status" :class="'ws-' + connectionState">
          <span class="ws-dot"></span>
          <span>{{ connectionLabel }}</span>
        </div>
      </div>
    </header>

    <!-- Stats Row -->
    <section class="stats-row">
      <div class="stat-card">
        <div
          class="stat-icon"
          style="background: var(--accent-soft); color: var(--accent)"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <rect x="2" y="2" width="20" height="8" rx="2" />
            <rect x="2" y="14" width="20" height="8" rx="2" />
          </svg>
        </div>
        <div class="stat-info">
          <span class="stat-value">{{ hosts.length }}</span>
          <span class="stat-label">{{ hostCountLabel }}</span>
        </div>
      </div>
      <div class="stat-card">
        <div
          class="stat-icon"
          style="background: var(--info-soft); color: var(--info)"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path
              d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"
            />
            <line x1="12" y1="9" x2="12" y2="13" />
            <line x1="12" y1="17" x2="12.01" y2="17" />
          </svg>
        </div>
        <div class="stat-info">
          <span class="stat-value">{{ alerts.length }}</span>
          <span class="stat-label">活跃告警</span>
        </div>
      </div>
      <div class="stat-card">
        <div
          class="stat-icon"
          style="background: rgba(99, 102, 241, 0.12); color: #818cf8"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path d="M3 12h4l3 8 4-16 3 8h4" />
          </svg>
        </div>
        <div class="stat-info">
          <span class="stat-value" style="color: #818cf8">{{ alertEvents.length }}</span>
          <span class="stat-label">最近事件</span>
        </div>
      </div>
      <div class="stat-card">
        <div
          class="stat-icon"
          style="background: var(--danger-soft); color: var(--danger)"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="12" cy="12" r="10" />
            <line x1="15" y1="9" x2="9" y2="15" />
            <line x1="9" y1="9" x2="15" y2="15" />
          </svg>
        </div>
        <div class="stat-info">
          <span class="stat-value" style="color: var(--danger)">{{ criticalCount }}</span>
          <span class="stat-label">严重</span>
        </div>
      </div>
      <div class="stat-card">
        <div
          class="stat-icon"
          style="background: var(--warning-soft); color: var(--warning)"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path
              d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"
            />
            <line x1="12" y1="9" x2="12" y2="13" />
            <line x1="12" y1="17" x2="12.01" y2="17" />
          </svg>
        </div>
        <div class="stat-info">
          <span class="stat-value" style="color: var(--warning)">{{ warningCount }}</span>
          <span class="stat-label">警告</span>
        </div>
      </div>
      <div class="stat-card">
        <div
          class="stat-icon"
          style="background: rgba(6, 182, 212, 0.12); color: var(--info)"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="12" cy="12" r="10" />
            <line x1="12" y1="16" x2="12" y2="12" />
            <line x1="12" y1="8" x2="12.01" y2="8" />
          </svg>
        </div>
        <div class="stat-info">
          <span class="stat-value" style="color: var(--info)">{{ infoCount }}</span>
          <span class="stat-label">提示</span>
        </div>
      </div>
    </section>

    <!-- Hosts Section -->
    <section class="panel">
      <div class="panel-header">
        <div class="panel-title">
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            style="color: var(--accent)"
          >
            <rect x="2" y="2" width="20" height="8" rx="2" />
            <rect x="2" y="14" width="20" height="8" rx="2" />
          </svg>
          <h2>主机指标</h2>
        </div>
        <div class="panel-actions panel-actions-wrap">
          <form class="search-form" @submit.prevent="applyHostSearch">
            <input
              v-model="hostSearchInput"
              type="text"
              class="search-input"
              placeholder="搜索主机名"
            />
            <button type="submit" class="search-btn">
              搜索
            </button>
          </form>
          <div class="filter-group">
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedHostStatus === 'all' }"
              @click="setHostStatusFilter('all')"
            >
              全部
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedHostStatus === 'up' }"
              @click="setHostStatusFilter('up')"
            >
              在线
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedHostStatus === 'down' }"
              @click="setHostStatusFilter('down')"
            >
              离线
            </button>
          </div>
          <div class="filter-group">
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedHostSort === 'instance' }"
              @click="setHostSort('instance')"
            >
              名称
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedHostSort === 'cpu_desc' }"
              @click="setHostSort('cpu_desc')"
            >
              CPU
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedHostSort === 'memory_desc' }"
              @click="setHostSort('memory_desc')"
            >
              内存
            </button>
          </div>
          <span class="panel-badge">WebSocket 实时推送</span>
        </div>
      </div>

      <!-- Skeleton Loading -->
      <div v-if="loading" class="hosts-grid">
        <div v-for="n in 3" :key="n" class="host-card skeleton">
          <div class="skeleton-header">
            <div class="skeleton-dot"></div>
            <div class="skeleton-line" style="width: 60%"></div>
          </div>
          <div class="skeleton-metric">
            <div class="skeleton-label"></div>
            <div class="skeleton-bar"></div>
            <div class="skeleton-value"></div>
          </div>
          <div class="skeleton-metric">
            <div class="skeleton-label"></div>
            <div class="skeleton-bar"></div>
            <div class="skeleton-value"></div>
          </div>
        </div>
      </div>

      <div v-else-if="hosts.length === 0" class="empty-state">
        <div class="empty-icon">
          <svg
            width="48"
            height="48"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <rect x="2" y="2" width="20" height="8" rx="2" />
            <rect x="2" y="14" width="20" height="8" rx="2" />
          </svg>
        </div>
        <p>
          {{
            appliedHostQuery
              ? "没有匹配的主机"
              : selectedHostStatus === "all"
                ? "暂无主机数据"
                : "当前筛选条件下没有主机"
          }}
        </p>
        <p class="empty-sub">
          {{
            appliedHostQuery
              ? `没有匹配“${hostSearchInput.trim() || appliedHostQuery}”的主机`
              : selectedHostStatus === "all"
                ? "Prometheus 尚未发现任何主机"
                : selectedHostStatus === "up"
                  ? "当前没有在线主机"
                  : "当前没有离线主机"
          }}
        </p>
      </div>
      <div v-else class="hosts-grid">
        <div v-for="host in hosts" :key="host.instance" class="host-card">
          <div class="host-header">
            <div class="host-name-row">
              <span
                class="status-dot"
                :class="isHostUp(host.status) ? 'dot-up' : 'dot-down'"
              ></span>
              <span class="host-name">{{ host.instance }}</span>
            </div>
            <span
              class="host-status"
              :class="isHostUp(host.status) ? 'status-up' : 'status-down'"
            >
              {{ isHostUp(host.status) ? "在线" : "离线" }}
            </span>
          </div>
          <div class="host-metrics">
            <div class="metric-row">
              <div class="metric-label">CPU</div>
              <div class="metric-bar-bg">
                <div
                  class="metric-bar-fill"
                  :style="{
                    width: Math.min(host.cpu, 100) + '%',
                    background: cpuColor(host.cpu),
                  }"
                />
              </div>
              <div class="metric-value" :style="{ color: cpuColor(host.cpu) }">
                {{ host.cpu.toFixed(1) }}%
              </div>
            </div>
            <div class="metric-row">
              <div class="metric-label">内存</div>
              <div class="metric-bar-bg">
                <div
                  class="metric-bar-fill"
                  :style="{
                    width: Math.min(host.memory, 100) + '%',
                    background: memoryColor(host.memory),
                  }"
                />
              </div>
              <div
                class="metric-value"
                :style="{ color: memoryColor(host.memory) }"
              >
                {{ host.memory.toFixed(1) }}%
              </div>
            </div>
          </div>
          <div class="host-footer">
            <svg
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <circle cx="12" cy="12" r="10" />
              <polyline points="12 6 12 12 16 14" />
            </svg>
            最后采集: {{ formatTime(host.lastScrape) }}
          </div>
        </div>
      </div>
    </section>

    <!-- Alerts Section -->
    <section class="panel">
      <div class="panel-header">
        <div class="panel-title">
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            style="color: var(--warning)"
          >
            <path
              d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"
            />
            <line x1="12" y1="9" x2="12" y2="13" />
            <line x1="12" y1="17" x2="12.01" y2="17" />
          </svg>
          <h2>告警列表</h2>
        </div>
        <div class="panel-actions">
          <div class="filter-group">
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedSeverity === 'all' }"
              @click="setSeverityFilter('all')"
            >
              全部
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedSeverity === 'critical' }"
              @click="setSeverityFilter('critical')"
            >
              严重
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedSeverity === 'warning' }"
              @click="setSeverityFilter('warning')"
            >
              警告
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedSeverity === 'info' }"
              @click="setSeverityFilter('info')"
            >
              提示
            </button>
          </div>
          <button
            type="button"
            class="refresh-btn"
            :disabled="refreshing"
            @click="refreshAll"
          >
            <svg
              v-if="!refreshing"
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <polyline points="23 4 23 10 17 10" />
              <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
            </svg>
            <span v-else class="spin"></span>
            {{ refreshing ? "刷新中..." : "刷新" }}
          </button>
        </div>
      </div>

      <div v-if="alertsError" class="error-banner">
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <circle cx="12" cy="12" r="10" />
          <line x1="15" y1="9" x2="9" y2="15" />
          <line x1="9" y1="9" x2="15" y2="15" />
        </svg>
        {{ alertsError }}
      </div>

      <div v-else-if="alerts.length === 0" class="empty-state">
        <div class="empty-icon" style="color: var(--success)">
          <svg
            width="48"
            height="48"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
            <polyline points="22 4 12 14.01 9 11.01" />
          </svg>
        </div>
        <p style="color: var(--success); font-weight: 600">一切正常</p>
        <p class="empty-sub">当前无活跃告警</p>
      </div>

      <div v-else-if="alerts.length === 0" class="empty-state">
        <p>该级别下无告警</p>
      </div>

      <div v-else class="alert-list">
        <div
          v-for="alert in alerts"
          :key="alert.fingerprint"
          class="alert-card"
          :class="severityClass(alert.labels.severity)"
        >
          <div class="alert-top">
            <span
              class="alert-severity"
              :class="severityClass(alert.labels.severity)"
            >
              {{ severityLabel(alert.labels.severity) }}
            </span>
            <span class="alert-time">{{ formatTime(alert.startsAt) }}</span>
          </div>
          <div class="alert-body">
            <div class="alert-name">
              {{ alert.labels.alertname || "未知告警" }}
            </div>
            <div class="alert-instance">
              {{ alert.labels.instance || "" }}
            </div>
            <p class="alert-summary">
              {{ alert.annotations.summary || alert.annotations.description || "" }}
            </p>
            <p
              v-if="alert.annotations.description && alert.annotations.summary"
              class="alert-desc"
            >
              {{ alert.annotations.description }}
            </p>
          </div>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="panel-header">
        <div class="panel-title">
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            style="color: #818cf8"
          >
            <path d="M3 12h4l3 8 4-16 3 8h4" />
          </svg>
          <h2>最近事件</h2>
        </div>
        <div class="panel-actions panel-actions-wrap">
          <div class="filter-group">
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedEventStatus === 'all' }"
              @click="setEventStatusFilter('all')"
            >
              全部状态
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedEventStatus === 'firing' }"
              @click="setEventStatusFilter('firing')"
            >
              触发
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedEventStatus === 'resolved' }"
              @click="setEventStatusFilter('resolved')"
            >
              恢复
            </button>
          </div>
          <div class="filter-group">
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedEventSeverity === 'all' }"
              @click="setEventSeverityFilter('all')"
            >
              全部级别
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedEventSeverity === 'critical' }"
              @click="setEventSeverityFilter('critical')"
            >
              严重
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedEventSeverity === 'warning' }"
              @click="setEventSeverityFilter('warning')"
            >
              警告
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: selectedEventSeverity === 'info' }"
              @click="setEventSeverityFilter('info')"
            >
              提示
            </button>
          </div>
          <span class="panel-badge event-badge">Webhook 历史流</span>
        </div>
      </div>

      <div v-if="alertEventsError" class="error-banner">
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <circle cx="12" cy="12" r="10" />
          <line x1="15" y1="9" x2="9" y2="15" />
          <line x1="9" y1="9" x2="15" y2="15" />
        </svg>
        {{ alertEventsError }}
      </div>

      <div v-else-if="alertEvents.length === 0" class="empty-state">
        <div class="empty-icon">
          <svg
            width="48"
            height="48"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path d="M3 12h4l3 8 4-16 3 8h4" />
          </svg>
        </div>
        <p>暂无最近事件</p>
        <p class="empty-sub">新告警或恢复事件会在这里按时间倒序展示</p>
      </div>

      <div v-else-if="alertEvents.length === 0" class="empty-state">
        <p>当前筛选条件下没有事件</p>
        <p class="empty-sub">可以切换状态或级别查看其他最近事件</p>
      </div>

      <div v-else class="event-list">
        <div
          v-for="event in latestAlertEvents"
          :key="`${event.fingerprint}-${event.receivedAt}-${event.status}`"
          class="event-card"
          :class="severityClass(event.labels.severity)"
        >
          <div class="event-top">
            <div class="event-top-left">
              <span
                class="alert-severity"
                :class="severityClass(event.labels.severity)"
              >
                {{ severityLabel(event.labels.severity) }}
              </span>
              <span class="event-status" :class="eventStatusClass(event.status)">
                {{ eventStatusLabel(event.status) }}
              </span>
            </div>
            <span class="alert-time">{{ formatTime(event.receivedAt) }}</span>
          </div>
          <div class="event-body">
            <div class="alert-name">
              {{ event.labels.alertname || "未知事件" }}
            </div>
            <div class="alert-instance">
              {{ event.labels.instance || "" }}
            </div>
            <p class="alert-summary">
              {{ event.annotations.summary || event.annotations.description || "" }}
            </p>
            <p class="event-meta">开始时间 {{ formatTime(event.startsAt) }}</p>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.app-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 1.5rem;
  min-height: 100vh;
}

/* Toast Notifications */
.toast-container {
  position: fixed;
  top: 1rem;
  right: 1rem;
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  pointer-events: none;
}

.toast {
  pointer-events: auto;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 0.75rem 1rem;
  min-width: 280px;
  max-width: 400px;
  box-shadow: var(--shadow-md);
  display: flex;
  align-items: center;
  gap: 0.5rem;
  backdrop-filter: blur(8px);
}

.toast.severity-critical {
  border-left: 3px solid var(--danger);
}

.toast.severity-warning {
  border-left: 3px solid var(--warning);
}

.toast.severity-info {
  border-left: 3px solid var(--info);
}

.toast-severity {
  font-size: 0.7rem;
  font-weight: 700;
  padding: 0.15em 0.4em;
  border-radius: 4px;
  flex-shrink: 0;
}

.toast.severity-critical .toast-severity {
  background: var(--danger-soft);
  color: var(--danger);
}

.toast.severity-warning .toast-severity {
  background: var(--warning-soft);
  color: var(--warning);
}

.toast.severity-info .toast-severity {
  background: var(--info-soft);
  color: var(--info);
}

.toast-message {
  font-size: 0.8rem;
  color: var(--text-secondary);
}

.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from {
  opacity: 0;
  transform: translateX(100%);
}

.toast-leave-to {
  opacity: 0;
  transform: translateX(100%);
}

/* Header */
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid var(--border-color);
}

.logo {
  display: flex;
  align-items: center;
  gap: 0.875rem;
}

.logo-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  background: linear-gradient(135deg, var(--accent), #6366f1);
  box-shadow: 0 0 16px var(--accent-glow);
  position: relative;
}

.logo-icon::after {
  content: "";
  position: absolute;
  inset: 8px;
  border: 2px solid rgba(255, 255, 255, 0.4);
  border-radius: 4px;
}

.logo-text h1 {
  font-size: 1.25rem;
  font-weight: 700;
  margin: 0;
  letter-spacing: -0.02em;
}

.logo-sub {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin: 0.15rem 0 0;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.update-ago {
  font-size: 0.7rem;
  color: var(--text-muted);
  font-weight: 500;
}

.clock {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.85rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--text-secondary);
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  padding: 0.4rem 0.75rem;
  border-radius: var(--radius-sm);
}

.clock svg {
  color: var(--accent);
}

.fullscreen-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
  color: var(--text-muted);
  background: var(--bg-card);
  transition: all 0.15s;
}

.fullscreen-btn:hover {
  border-color: var(--accent);
  color: var(--accent);
}

.ws-status {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.35rem 0.75rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
}

.ws-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.ws-connected {
  background: var(--success-soft);
  border-color: rgba(34, 197, 94, 0.3);
  color: var(--success);
}

.ws-connected .ws-dot {
  background: var(--success);
  box-shadow: 0 0 6px var(--success);
}

.ws-connecting {
  background: var(--warning-soft);
  border-color: rgba(245, 158, 11, 0.3);
  color: var(--warning);
}

.ws-connecting .ws-dot {
  background: var(--warning);
  animation: pulse 1.5s infinite;
}

.ws-disconnected {
  background: var(--danger-soft);
  border-color: rgba(239, 68, 68, 0.3);
  color: var(--danger);
}

.ws-disconnected .ws-dot {
  background: var(--danger);
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.4;
  }
}

/* Stats Row */
.stats-row {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.stat-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 1rem;
  display: flex;
  align-items: center;
  gap: 0.875rem;
  transition: all 0.2s ease;
}

.stat-card:hover {
  border-color: var(--border-hover);
  transform: translateY(-1px);
  box-shadow: var(--shadow-md);
}

.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-info {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1;
  color: var(--text-primary);
}

.stat-label {
  font-size: 0.7rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

/* Panel */
.panel {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: 1.25rem 1.5rem;
  margin-bottom: 1.5rem;
  backdrop-filter: blur(8px);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.25rem;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.panel-title {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.panel-title h2 {
  font-size: 1rem;
  font-weight: 600;
  margin: 0;
}

.panel-badge {
  font-size: 0.7rem;
  font-weight: 500;
  color: var(--accent);
  background: var(--accent-soft);
  padding: 0.2rem 0.6rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgba(59, 130, 246, 0.2);
}

.event-badge {
  color: #818cf8;
  background: rgba(99, 102, 241, 0.12);
  border-color: rgba(129, 140, 248, 0.22);
}

.panel-actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.panel-actions-wrap {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.search-form {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.25rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
  background: var(--bg-secondary);
}

.search-input {
  min-width: 11rem;
  padding: 0.35rem 0.5rem;
  color: var(--text-primary);
  cursor: text;
}

.search-input::placeholder {
  color: var(--text-muted);
}

.search-btn {
  padding: 0.35rem 0.75rem;
  border-radius: var(--radius-sm);
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 0.75rem;
  font-weight: 600;
  transition: all 0.15s ease;
}

.search-btn:hover {
  background: rgba(59, 130, 246, 0.18);
}

/* Skeleton */
.skeleton {
  animation: skeleton-pulse 1.5s ease-in-out infinite;
}

@keyframes skeleton-pulse {
  0%,
  100% {
    opacity: 0.6;
  }
  50% {
    opacity: 1;
  }
}

.skeleton-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.skeleton-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--border-color);
}

.skeleton-line {
  height: 16px;
  background: var(--border-color);
  border-radius: 4px;
}

.skeleton-metric {
  display: grid;
  grid-template-columns: 2.5rem 1fr 3.5rem;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}

.skeleton-label {
  height: 12px;
  background: var(--border-color);
  border-radius: 3px;
}

.skeleton-bar {
  height: 8px;
  background: var(--border-color);
  border-radius: 4px;
}

.skeleton-value {
  height: 12px;
  background: var(--border-color);
  border-radius: 3px;
}

/* Hosts Grid */
.hosts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1rem;
}

.host-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 1rem;
  transition: all 0.2s ease;
  position: relative;
  overflow: hidden;
}

.host-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(
    90deg,
    transparent,
    rgba(255, 255, 255, 0.02),
    transparent
  );
  transition: left 0.5s ease;
}

.host-card:hover::before {
  left: 100%;
}

.host-card:hover {
  border-color: var(--border-hover);
  box-shadow: var(--shadow-md);
}

.host-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.host-name-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-up {
  background: var(--success);
  box-shadow: 0 0 6px var(--success);
}

.dot-down {
  background: var(--danger);
  box-shadow: 0 0 6px var(--danger);
}

.host-name {
  font-weight: 600;
  font-size: 0.9rem;
}

.host-status {
  font-size: 0.7rem;
  font-weight: 600;
  padding: 0.2em 0.6em;
  border-radius: var(--radius-sm);
}

.status-up {
  background: var(--success-soft);
  color: var(--success);
}

.status-down {
  background: var(--danger-soft);
  color: var(--danger);
}

.host-metrics {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.metric-row {
  display: grid;
  grid-template-columns: 2.5rem 1fr 3.5rem;
  align-items: center;
  gap: 0.75rem;
}

.metric-label {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-weight: 500;
}

.metric-bar-bg {
  height: 8px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 4px;
  overflow: hidden;
}

.metric-bar-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.6s ease;
}

.metric-value {
  font-size: 0.8rem;
  font-weight: 600;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.host-footer {
  margin-top: 0.875rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--border-color);
  font-size: 0.7rem;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

/* Filter */
.filter-group {
  display: flex;
  gap: 0.25rem;
  background: var(--bg-secondary);
  padding: 0.25rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
}

.filter-btn {
  font-size: 0.75rem;
  font-weight: 500;
  padding: 0.35em 0.75em;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  transition: all 0.15s;
}

.filter-btn:hover {
  color: var(--text-secondary);
}

.filter-btn.active {
  background: var(--accent-soft);
  color: var(--accent);
  font-weight: 600;
}

/* Refresh Button */
.refresh-btn {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.75rem;
  font-weight: 500;
  padding: 0.4rem 0.75rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
  background: var(--bg-secondary);
  transition: all 0.15s;
}

.refresh-btn:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent);
}

.refresh-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.spin {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border-color);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

/* Empty State */
.empty-state {
  text-align: center;
  padding: 3rem 0;
  color: var(--text-muted);
}

.empty-icon {
  margin-bottom: 1rem;
  color: var(--text-muted);
}

.empty-sub {
  font-size: 0.8rem;
  margin-top: 0.35rem;
}

/* Error Banner */
.error-banner {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background: var(--danger-soft);
  border: 1px solid rgba(239, 68, 68, 0.25);
  border-radius: var(--radius-sm);
  padding: 0.75rem 1rem;
  color: var(--danger);
  font-size: 0.85rem;
}

/* Alert List */
.alert-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.alert-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 1rem;
  transition: all 0.2s ease;
  border-left: 3px solid transparent;
}

.alert-card:hover {
  border-color: var(--border-hover);
  box-shadow: var(--shadow-md);
}

.alert-card.severity-critical {
  border-left-color: var(--danger);
}

.alert-card.severity-warning {
  border-left-color: var(--warning);
}

.alert-card.severity-info {
  border-left-color: var(--info);
}

.alert-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
}

.alert-severity {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  padding: 0.2em 0.6em;
  border-radius: var(--radius-sm);
}

.alert-severity.severity-critical {
  background: var(--danger-soft);
  color: var(--danger);
}

.alert-severity.severity-warning {
  background: var(--warning-soft);
  color: var(--warning);
}

.alert-severity.severity-info {
  background: var(--info-soft);
  color: var(--info);
}

.alert-time {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.alert-name {
  font-weight: 600;
  font-size: 0.9rem;
  margin-bottom: 0.15rem;
}

.alert-instance {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin-bottom: 0.5rem;
}

.alert-summary {
  font-size: 0.8rem;
  color: var(--text-secondary);
  margin: 0;
}

.alert-desc {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin: 0.35rem 0 0;
}

.event-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 0.75rem;
}

.event-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 1rem;
  transition: all 0.2s ease;
  border-left: 3px solid transparent;
}

.event-card:hover {
  border-color: var(--border-hover);
  box-shadow: var(--shadow-md);
}

.event-card.severity-critical {
  border-left-color: var(--danger);
}

.event-card.severity-warning {
  border-left-color: var(--warning);
}

.event-card.severity-info {
  border-left-color: var(--info);
}

.event-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.65rem;
}

.event-top-left {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.event-status {
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  padding: 0.2em 0.55em;
  border-radius: var(--radius-sm);
}

.event-status-firing {
  background: rgba(239, 68, 68, 0.1);
  color: var(--danger);
}

.event-status-resolved {
  background: rgba(34, 197, 94, 0.12);
  color: var(--success);
}

.event-meta {
  font-size: 0.72rem;
  color: var(--text-muted);
  margin-top: 0.45rem;
}

/* Responsive */
@media (max-width: 768px) {
  .app-container {
    padding: 1rem;
  }

  .header {
    flex-direction: column;
    align-items: flex-start;
    gap: 1rem;
  }

  .header-right {
    flex-wrap: wrap;
    width: 100%;
  }

  .stats-row {
    grid-template-columns: repeat(3, 1fr);
  }

  .panel-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .panel-actions {
    width: 100%;
    justify-content: space-between;
  }

  .panel-actions-wrap {
    justify-content: flex-start;
  }

  .search-form {
    width: 100%;
  }

  .search-input {
    min-width: 0;
    flex: 1;
  }

  .hosts-grid {
    grid-template-columns: 1fr;
  }

  .toast-container {
    left: 1rem;
    right: 1rem;
  }

  .toast {
    max-width: 100%;
    min-width: auto;
  }
}

@media (max-width: 480px) {
  .stats-row {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
