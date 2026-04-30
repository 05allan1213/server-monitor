<script setup lang="ts">
import HostsPanel from "../components/HostsPanel.vue";
import { useMonitorStore } from "../stores/monitor";

const monitor = useMonitorStore();
</script>

<template>
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
