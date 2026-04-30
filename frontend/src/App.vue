<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from "vue";

import { fetchActiveAlerts, fetchAlertEvents } from "./api/alerts";
import { fetchHosts } from "./api/hosts";
import AlertsPanel from "./components/AlertsPanel.vue";
import HostsPanel from "./components/HostsPanel.vue";
import HostResourceChart from "./components/HostResourceChart.vue";
import StatsRow from "./components/StatsRow.vue";
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
const selectedHostRisk = ref<"all" | "high_cpu" | "high_memory">("all");
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
const highCPUHostCount = computed(() => hosts.value.filter((host) => isHighCPU(host)).length);
const highMemoryHostCount = computed(() => hosts.value.filter((host) => isHighMemory(host)).length);
const bothRiskHostCount = computed(() => hosts.value.filter((host) => hostRiskVariant(host) === "both").length);
const hostFilterSummary = computed(() => {
  const parts: string[] = [];

  if (selectedHostStatus.value === "up") {
    parts.push("在线");
  } else if (selectedHostStatus.value === "down") {
    parts.push("离线");
  }

  if (appliedHostQuery.value) {
    parts.push(`搜索: ${appliedHostQuery.value}`);
  }

  switch (selectedHostSort.value) {
    case "cpu_desc":
      parts.push("按 CPU 排序");
      break;
    case "memory_desc":
      parts.push("按内存排序");
      break;
  }

  switch (selectedHostRisk.value) {
    case "high_cpu":
      parts.push("高 CPU");
      break;
    case "high_memory":
      parts.push("高内存");
      break;
  }

  return parts;
});
const hasActiveHostFilters = computed(() => hostFilterSummary.value.length > 0);
const hostViewSummary = computed(() => {
  if (loading.value) {
    return "主机视图加载中";
  }

  return hasActiveHostFilters.value
    ? `当前视图匹配 ${hosts.value.length} 台主机`
    : `当前展示 ${hosts.value.length} 台主机`;
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

function isHighCPU(host: Host): boolean {
  return host.cpu >= 80;
}

function isHighMemory(host: Host): boolean {
  return host.memory >= 85;
}

function hostRiskVariant(host: Host): "normal" | "cpu" | "memory" | "both" {
  const highCPU = isHighCPU(host);
  const highMemory = isHighMemory(host);

  if (highCPU && highMemory) {
    return "both";
  }
  if (highCPU) {
    return "cpu";
  }
  if (highMemory) {
    return "memory";
  }

  return "normal";
}

function matchesHostQuery(host: Host, query: string): boolean {
  if (!query) {
    return true;
  }

  return host.instance.toLowerCase().includes(query);
}

function isHostUp(status: string): boolean {
  return status === "up";
}

function matchesHostRisk(host: Host): boolean {
  switch (selectedHostRisk.value) {
    case "high_cpu":
      return host.cpu >= 80;
    case "high_memory":
      return host.memory >= 85;
    default:
      return true;
  }
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

function setHostRisk(value: "all" | "high_cpu" | "high_memory") {
  selectedHostRisk.value = value;
  loadHosts();
}

function applyHostSearch() {
  appliedHostQuery.value = hostSearchInput.value.trim().toLowerCase();
  loadHosts();
}

function resetHostFilters() {
  hostSearchInput.value = "";
  appliedHostQuery.value = "";
  selectedHostStatus.value = "all";
  selectedHostSort.value = "instance";
  selectedHostRisk.value = "all";
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
      risk: selectedHostRisk.value,
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

    return statusMatched && matchesHostQuery(host, appliedHostQuery.value) && matchesHostRisk(host);
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

    <StatsRow
      :host-count="hosts.length"
      :host-count-label="hostCountLabel"
      :high-cpu-host-count="highCPUHostCount"
      :high-memory-host-count="highMemoryHostCount"
      :both-risk-host-count="bothRiskHostCount"
      :active-alert-count="alerts.length"
      :alert-event-count="alertEvents.length"
      :critical-count="criticalCount"
      :warning-count="warningCount"
      :info-count="infoCount"
    />

    <!-- Host Resource Chart -->
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
            style="color: var(--info)"
          >
            <path d="M3 3v18h18" />
            <rect x="7" y="10" width="3" height="7" rx="1" />
            <rect x="12" y="6" width="3" height="11" rx="1" />
            <rect x="17" y="13" width="3" height="4" rx="1" />
          </svg>
          <h2>资源分布</h2>
        </div>
        <span class="panel-badge">ECharts</span>
      </div>
      <HostResourceChart :hosts="hosts" />
    </section>

    <HostsPanel
      :hosts="hosts"
      :loading="loading"
      :host-search-input="hostSearchInput"
      :applied-host-query="appliedHostQuery"
      :selected-host-status="selectedHostStatus"
      :selected-host-sort="selectedHostSort"
      :selected-host-risk="selectedHostRisk"
      :host-view-summary="hostViewSummary"
      :host-filter-summary="hostFilterSummary"
      :has-active-host-filters="hasActiveHostFilters"
      @update:host-search-input="hostSearchInput = $event"
      @apply-search="applyHostSearch"
      @status-change="setHostStatusFilter"
      @sort-change="setHostSort"
      @risk-change="setHostRisk"
      @reset-filters="resetHostFilters"
    />

    <AlertsPanel
      :alerts="alerts"
      :selected-severity="selectedSeverity"
      :refreshing="refreshing"
      :error="alertsError"
      @severity-change="setSeverityFilter"
      @refresh="refreshAll"
    />

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

  .toast-container {
    left: 1rem;
    right: 1rem;
  }

  .toast {
    max-width: 100%;
    min-width: auto;
  }
}

</style>
