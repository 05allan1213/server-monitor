<script setup lang="ts">
import HostsPanel from "../components/HostsPanel.vue";
import { useMonitorStore } from "../stores/monitor";

const monitor = useMonitorStore();
</script>

<template>
  <div v-if="monitor.hostsError" class="hosts-error">
    {{ monitor.hostsError }}
  </div>
  <HostsPanel
    :hosts="monitor.hosts"
    :loading="monitor.loading"
    :host-search-input="monitor.hostSearchInput"
    :applied-host-query="monitor.appliedHostQuery"
    :selected-host-status="monitor.selectedHostStatus"
    :selected-host-sort="monitor.selectedHostSort"
    :selected-host-risk="monitor.selectedHostRisk"
    :host-view-summary="monitor.hostViewSummary"
    :host-filter-summary="monitor.hostFilterSummary"
    :has-active-host-filters="monitor.hasActiveHostFilters"
    @update:host-search-input="monitor.hostSearchInput = $event"
    @apply-search="monitor.applyHostSearch"
    @status-change="monitor.setHostStatusFilter"
    @sort-change="monitor.setHostSort"
    @risk-change="monitor.setHostRisk"
    @reset-filters="monitor.resetHostFilters"
  />
</template>

<style scoped>
.hosts-error {
  margin-bottom: 1rem;
  color: var(--danger);
  background: var(--danger-soft);
  border: 1px solid rgba(239, 68, 68, 0.24);
  border-radius: var(--radius-md);
  padding: 0.75rem 1rem;
  font-size: 0.82rem;
}
</style>
