<script setup lang="ts">
import HostsPanel from "../components/HostsPanel.vue";
import type { Host } from "../types";

type HostStatusFilter = "all" | "up" | "down";
type HostSort = "instance" | "cpu_desc" | "memory_desc";
type HostRiskFilter = "all" | "high_cpu" | "high_memory";

defineProps<{
  hosts: Host[];
  loading: boolean;
  hostSearchInput: string;
  appliedHostQuery: string;
  selectedHostStatus: HostStatusFilter;
  selectedHostSort: HostSort;
  selectedHostRisk: HostRiskFilter;
  hostViewSummary: string;
  hostFilterSummary: string[];
  hasActiveHostFilters: boolean;
}>();

const emit = defineEmits<{
  "update:hostSearchInput": [value: string];
  applySearch: [];
  statusChange: [value: HostStatusFilter];
  sortChange: [value: HostSort];
  riskChange: [value: HostRiskFilter];
  resetFilters: [];
}>();
</script>

<template>
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
    @update:host-search-input="emit('update:hostSearchInput', $event)"
    @apply-search="emit('applySearch')"
    @status-change="emit('statusChange', $event)"
    @sort-change="emit('sortChange', $event)"
    @risk-change="emit('riskChange', $event)"
    @reset-filters="emit('resetFilters')"
  />
</template>
