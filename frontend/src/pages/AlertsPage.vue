<script setup lang="ts">
import AlertEventsPanel from "../components/AlertEventsPanel.vue";
import AlertsPanel from "../components/AlertsPanel.vue";
import { useMonitorStore } from "../stores/monitor";

const monitor = useMonitorStore();
</script>

<template>
  <AlertsPanel
    :alerts="monitor.alerts"
    :selected-severity="monitor.selectedSeverity"
    :refreshing="monitor.refreshing"
    :error="monitor.alertsError"
    @severity-change="monitor.setSeverityFilter"
    @refresh="monitor.refreshAll"
  />

  <AlertEventsPanel
    :events="monitor.latestAlertEvents"
    :selected-status="monitor.selectedEventStatus"
    :selected-severity="monitor.selectedEventSeverity"
    :error="monitor.alertEventsError"
    @status-change="monitor.setEventStatusFilter"
    @severity-change="monitor.setEventSeverityFilter"
  />
</template>
