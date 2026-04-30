<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from "vue";
import { RouterLink, RouterView } from "vue-router";

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

    <nav class="route-tabs" aria-label="页面导航">
      <RouterLink to="/" class="route-tab" exact-active-class="active">
        总览
      </RouterLink>
      <RouterLink to="/hosts" class="route-tab" exact-active-class="active">
        主机
      </RouterLink>
      <RouterLink to="/alerts" class="route-tab" exact-active-class="active">
        告警
      </RouterLink>
    </nav>

    <RouterView v-slot="{ Component, route }">
      <component
        :is="Component"
        v-if="route.name === 'overview'"
        :hosts="hosts"
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
      <component
        :is="Component"
        v-else-if="route.name === 'hosts'"
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
      <component
        :is="Component"
        v-else
        :alerts="alerts"
        :events="latestAlertEvents"
        :selected-severity="selectedSeverity"
        :refreshing="refreshing"
        :alerts-error="alertsError"
        :selected-event-status="selectedEventStatus"
        :selected-event-severity="selectedEventSeverity"
        :alert-events-error="alertEventsError"
        @severity-change="setSeverityFilter"
        @refresh="refreshAll"
        @event-status-change="setEventStatusFilter"
        @event-severity-change="setEventSeverityFilter"
      />
    </RouterView>
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

.route-tabs {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  margin-bottom: 1.5rem;
  border-bottom: 1px solid var(--border-color);
}

.route-tab {
  color: var(--text-muted);
  font-size: 0.85rem;
  font-weight: 600;
  padding: 0.7rem 0.9rem;
  border-bottom: 2px solid transparent;
  transition: color 0.15s, border-color 0.15s;
}

.route-tab:hover {
  color: var(--text-secondary);
}

.route-tab.active {
  color: var(--accent);
  border-bottom-color: var(--accent);
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
