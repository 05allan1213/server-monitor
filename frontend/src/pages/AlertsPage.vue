<script setup lang="ts">
import AlertEventsPanel from "../components/AlertEventsPanel.vue";
import AlertsPanel from "../components/AlertsPanel.vue";
import type { AlertEvent, AlertRecord } from "../types";

type EventStatusFilter = "all" | "firing" | "resolved";
type SeverityFilter = "all" | "critical" | "warning" | "info";

defineProps<{
  alerts: AlertRecord[];
  events: AlertEvent[];
  selectedSeverity: SeverityFilter;
  refreshing: boolean;
  alertsError: string;
  selectedEventStatus: EventStatusFilter;
  selectedEventSeverity: SeverityFilter;
  alertEventsError: string;
}>();

const emit = defineEmits<{
  severityChange: [value: SeverityFilter];
  refresh: [];
  eventStatusChange: [value: EventStatusFilter];
  eventSeverityChange: [value: SeverityFilter];
}>();
</script>

<template>
  <AlertsPanel
    :alerts="alerts"
    :selected-severity="selectedSeverity"
    :refreshing="refreshing"
    :error="alertsError"
    @severity-change="emit('severityChange', $event)"
    @refresh="emit('refresh')"
  />

  <AlertEventsPanel
    :events="events"
    :selected-status="selectedEventStatus"
    :selected-severity="selectedEventSeverity"
    :error="alertEventsError"
    @status-change="emit('eventStatusChange', $event)"
    @severity-change="emit('eventSeverityChange', $event)"
  />
</template>
