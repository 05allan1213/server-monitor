<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

import { fetchActiveAlerts } from "./api/alerts";
import { useAlertsWebSocket } from "./composables/useAlertsWebSocket";
import type { AlertRecord } from "./types";

const alerts = ref<AlertRecord[]>([]);
const loading = ref(true);
const refreshing = ref(false);
const errorMessage = ref("");
const lastUpdatedAt = ref("");
const latestEvent = ref("");
const selectedSeverity = ref<"all" | "critical" | "warning">("all");

const criticalCount = computed(
  () => alerts.value.filter((alert) => alert.labels.severity === "critical").length,
);

const warningCount = computed(
  () => alerts.value.filter((alert) => alert.labels.severity === "warning").length,
);

const filteredAlerts = computed(() => {
  if (selectedSeverity.value === "all") {
    return alerts.value;
  }

  return alerts.value.filter(
    (alert) => (alert.labels.severity ?? "info") === selectedSeverity.value,
  );
});

const connectionLabel = computed(() => {
  switch (connectionState.value) {
    case "connected":
      return "Live";
    case "connecting":
      return "Connecting";
    default:
      return "Offline";
  }
});

async function loadAlerts(showSpinner = true) {
  if (showSpinner) {
    loading.value = true;
  } else {
    refreshing.value = true;
  }

  errorMessage.value = "";

  try {
    alerts.value = await fetchActiveAlerts();
    lastUpdatedAt.value = new Date().toLocaleString();
  } catch (error) {
    errorMessage.value =
      error instanceof Error ? error.message : "Failed to load active alerts";
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
}

function applyIncomingAlert(alert: AlertRecord) {
  latestEvent.value =
    alert.status === "resolved"
      ? `Resolved: ${alert.labels.alertname ?? alert.fingerprint}`
      : `Firing: ${alert.labels.alertname ?? alert.fingerprint}`;
  lastUpdatedAt.value = new Date().toLocaleString();

  const nextAlerts = [...alerts.value];
  const existingIndex = nextAlerts.findIndex(
    (item) => item.fingerprint === alert.fingerprint,
  );

  if (alert.status === "resolved") {
    if (existingIndex >= 0) {
      nextAlerts.splice(existingIndex, 1);
    }
    alerts.value = nextAlerts;
    return;
  }

  if (existingIndex >= 0) {
    nextAlerts.splice(existingIndex, 1);
  }

  nextAlerts.unshift(alert);
  alerts.value = nextAlerts.sort(
    (left, right) =>
      new Date(right.startsAt).getTime() - new Date(left.startsAt).getTime(),
  );
}

const { connectionState, connect } = useAlertsWebSocket(applyIncomingAlert);

function formatTime(value: string) {
  if (!value) {
    return "Unknown";
  }

  return new Date(value).toLocaleString();
}

function setSeverityFilter(value: "all" | "critical" | "warning") {
  selectedSeverity.value = value;
}

onMounted(() => {
  void loadAlerts();
  connect();
});
</script>

<template>
  <main class="shell">
    <section class="hero">
      <div class="hero-copy">
        <p class="eyebrow">Stage 3.9</p>
        <h1>Active alert console</h1>
        <p class="lede">
          This page now consumes <code>/api/v1/alerts/active</code> for initial state and
          keeps the alert feed fresh through <code>/ws/alerts</code>.
        </p>

        <div class="hero-badges">
          <span class="hero-badge">
            <span class="hero-badge-dot hero-badge-dot-live" />
            {{ connectionLabel }}
          </span>
          <span class="hero-badge">
            <span class="hero-badge-dot hero-badge-dot-alert" />
            {{ latestEvent || "Waiting for next alert event" }}
          </span>
        </div>
      </div>

      <div class="hero-stats">
        <article class="stat-card">
          <span class="stat-label">Active alerts</span>
          <strong class="stat-value">{{ alerts.length }}</strong>
        </article>
        <article class="stat-card stat-card-critical">
          <span class="stat-label">Critical</span>
          <strong class="stat-value">{{ criticalCount }}</strong>
        </article>
        <article class="stat-card stat-card-warning">
          <span class="stat-label">Warning</span>
          <strong class="stat-value">{{ warningCount }}</strong>
        </article>
        <article class="stat-card" :class="`stat-card-${connectionState}`">
          <span class="stat-label">WebSocket</span>
          <strong class="stat-value">{{ connectionLabel }}</strong>
        </article>
      </div>
    </section>

    <section class="panel">
      <header class="panel-header">
        <div class="panel-heading">
          <h2>Alert feed</h2>
          <p class="panel-meta">
            {{ filteredAlerts.length }} shown
            <span v-if="lastUpdatedAt"> · Last updated: {{ lastUpdatedAt }}</span>
          </p>
          <p v-if="latestEvent" class="panel-meta panel-meta-event">{{ latestEvent }}</p>
        </div>

        <div class="panel-actions">
          <div class="filter-pill-group">
            <button
              type="button"
              class="filter-pill"
              :class="{ 'filter-pill-active': selectedSeverity === 'all' }"
              @click="setSeverityFilter('all')"
            >
              All
            </button>
            <button
              type="button"
              class="filter-pill"
              :class="{ 'filter-pill-active': selectedSeverity === 'critical' }"
              @click="setSeverityFilter('critical')"
            >
              Critical
            </button>
            <button
              type="button"
              class="filter-pill"
              :class="{ 'filter-pill-active': selectedSeverity === 'warning' }"
              @click="setSeverityFilter('warning')"
            >
              Warning
            </button>
          </div>

          <button class="refresh-button" type="button" @click="loadAlerts(false)" :disabled="refreshing">
            {{ refreshing ? "Refreshing..." : "Refresh" }}
          </button>
        </div>
      </header>

      <div v-if="loading" class="state-card">Loading alerts...</div>
      <div v-else-if="errorMessage" class="state-card state-card-error">{{ errorMessage }}</div>
      <div v-else-if="alerts.length === 0" class="state-card state-card-empty">
        <strong>Everything looks calm.</strong>
        <span>No active alerts right now.</span>
      </div>
      <div v-else-if="filteredAlerts.length === 0" class="state-card state-card-empty">
        <strong>No alerts in this filter.</strong>
        <span>Try switching back to All to see the full active set.</span>
      </div>

      <div v-else class="alert-grid">
        <article
          v-for="alert in filteredAlerts"
          :key="alert.fingerprint"
          class="alert-card"
          :class="`alert-card-${alert.labels.severity ?? 'info'}`"
        >
          <header class="alert-card-header">
            <div>
              <p class="alert-name">{{ alert.labels.alertname ?? "UnknownAlert" }}</p>
              <p class="alert-instance">{{ alert.labels.instance ?? "unknown-instance" }}</p>
            </div>
            <span class="severity-pill">{{ alert.labels.severity ?? "info" }}</span>
          </header>

          <p class="alert-summary">
            {{ alert.annotations.summary ?? "No summary provided." }}
          </p>

          <dl class="alert-meta">
            <div>
              <dt>Status</dt>
              <dd class="status-value">{{ alert.status }}</dd>
            </div>
            <div>
              <dt>Started</dt>
              <dd>{{ formatTime(alert.startsAt) }}</dd>
            </div>
            <div>
              <dt>Fingerprint</dt>
              <dd class="fingerprint">{{ alert.fingerprint }}</dd>
            </div>
          </dl>
        </article>
      </div>
    </section>
  </main>
</template>

<style scoped>
.shell {
  width: min(1120px, calc(100vw - 32px));
  margin: 0 auto;
  padding: 40px 0 56px;
}

.hero {
  display: grid;
  gap: 24px;
  align-items: end;
  grid-template-columns: 1.2fr 0.8fr;
  margin-bottom: 28px;
}

.eyebrow {
  margin: 0 0 10px;
  color: var(--info);
  font-size: 0.88rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.hero-copy h1 {
  margin: 0;
  font-size: clamp(2.4rem, 6vw, 4.8rem);
  line-height: 0.96;
  letter-spacing: -0.05em;
}

.lede {
  max-width: 58ch;
  margin: 16px 0 0;
  color: var(--muted);
  font-size: 1rem;
  line-height: 1.7;
}

.lede code {
  color: #ffe4bf;
}

.hero-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 20px;
}

.hero-badge {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  max-width: 100%;
  padding: 10px 14px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 999px;
  background: rgba(10, 14, 18, 0.42);
  color: #f8f4ea;
  font-size: 0.9rem;
}

.hero-badge-dot {
  width: 9px;
  height: 9px;
  border-radius: 999px;
  flex: 0 0 auto;
}

.hero-badge-dot-live {
  background: var(--success);
  box-shadow: 0 0 0 6px rgba(104, 211, 145, 0.12);
}

.hero-badge-dot-alert {
  background: var(--accent);
  box-shadow: 0 0 0 6px rgba(255, 143, 67, 0.12);
}

.hero-stats {
  display: grid;
  gap: 14px;
}

.stat-card,
.panel,
.state-card,
.alert-card {
  border: 1px solid var(--panel-border);
  background: var(--panel);
  backdrop-filter: blur(16px);
  box-shadow: 0 18px 40px rgba(5, 8, 12, 0.22);
}

.stat-card {
  padding: 20px 22px;
  border-radius: 20px;
}

.stat-card-critical {
  border-color: rgba(255, 92, 87, 0.34);
}

.stat-card-warning {
  border-color: rgba(255, 143, 67, 0.32);
}

.stat-card-connected {
  border-color: rgba(104, 211, 145, 0.32);
}

.stat-card-connecting {
  border-color: rgba(124, 199, 226, 0.32);
}

.stat-card-disconnected {
  border-color: rgba(255, 92, 87, 0.26);
}

.stat-label {
  display: block;
  color: var(--muted);
  font-size: 0.92rem;
}

.stat-value {
  display: block;
  margin-top: 8px;
  font-size: 2rem;
  line-height: 1;
}

.panel {
  padding: 26px;
  border-radius: 28px;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 22px;
}

.panel-heading {
  min-width: 0;
}

.panel-actions {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.filter-pill-group {
  display: inline-flex;
  gap: 8px;
  padding: 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.filter-pill {
  border: 0;
  border-radius: 999px;
  padding: 10px 14px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  transition: background-color 160ms ease, color 160ms ease;
}

.filter-pill-active {
  background: rgba(255, 143, 67, 0.16);
  color: #ffe2ba;
}

.panel-header h2 {
  margin: 0;
  font-size: 1.5rem;
}

.panel-meta {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 0.92rem;
}

.panel-meta-event {
  color: #ffe4bf;
}

.refresh-button {
  border: 0;
  border-radius: 999px;
  padding: 12px 18px;
  color: #1a140d;
  background: linear-gradient(135deg, #ffcf90 0%, #ff8f43 100%);
  cursor: pointer;
  font-weight: 700;
}

.refresh-button:disabled {
  opacity: 0.62;
  cursor: wait;
}

.state-card {
  border-radius: 20px;
  padding: 26px 22px;
  color: var(--muted);
  display: grid;
  gap: 8px;
}

.state-card-error {
  border-color: rgba(255, 92, 87, 0.38);
  color: #ffd1cf;
}

.state-card-empty strong {
  color: #fff2d8;
  font-size: 1.05rem;
}

.alert-grid {
  display: grid;
  gap: 18px;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
}

.alert-card {
  border-radius: 22px;
  padding: 20px;
  position: relative;
  overflow: hidden;
}

.alert-card::after {
  content: "";
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.05), transparent 36%);
  pointer-events: none;
}

.alert-card-critical {
  border-color: rgba(255, 92, 87, 0.44);
}

.alert-card-warning {
  border-color: rgba(255, 143, 67, 0.44);
}

.alert-card-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.alert-name {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 700;
}

.alert-instance {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 0.92rem;
}

.severity-pill {
  border-radius: 999px;
  padding: 7px 10px;
  background: var(--accent-soft);
  color: #ffd7ae;
  font-size: 0.76rem;
  font-weight: 700;
  text-transform: uppercase;
}

.alert-summary {
  margin: 18px 0;
  color: #f6efe3;
  line-height: 1.65;
}

.alert-meta {
  display: grid;
  gap: 12px;
  margin: 0;
}

.alert-meta div {
  display: grid;
  gap: 4px;
}

.alert-meta dt {
  color: var(--muted);
  font-size: 0.82rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.alert-meta dd {
  margin: 0;
}

.status-value {
  text-transform: capitalize;
}

.fingerprint {
  word-break: break-all;
  color: #ffe9c8;
  font-size: 0.92rem;
}

@media (max-width: 860px) {
  .hero {
    grid-template-columns: 1fr;
  }

  .panel-header {
    flex-direction: column;
    align-items: stretch;
  }

  .refresh-button {
    width: 100%;
  }

  .panel-actions {
    width: 100%;
    align-items: stretch;
  }

  .filter-pill-group {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
