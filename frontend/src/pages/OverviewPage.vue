<script setup lang="ts">
import HostResourceChart from "../components/HostResourceChart.vue";
import StatsRow from "../components/StatsRow.vue";
import type { Host } from "../types";

defineProps<{
  hosts: Host[];
  hostCountLabel: string;
  highCPUHostCount: number;
  highMemoryHostCount: number;
  bothRiskHostCount: number;
  activeAlertCount: number;
  alertEventCount: number;
  criticalCount: number;
  warningCount: number;
  infoCount: number;
}>();
</script>

<template>
  <StatsRow
    :host-count="hosts.length"
    :host-count-label="hostCountLabel"
    :high-cpu-host-count="highCPUHostCount"
    :high-memory-host-count="highMemoryHostCount"
    :both-risk-host-count="bothRiskHostCount"
    :active-alert-count="activeAlertCount"
    :alert-event-count="alertEventCount"
    :critical-count="criticalCount"
    :warning-count="warningCount"
    :info-count="infoCount"
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
</template>

<style scoped>
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

@media (max-width: 768px) {
  .panel-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
