import { onBeforeUnmount, ref } from "vue";

import type { AlertRecord } from "../types";

type ConnectionState = "connecting" | "connected" | "disconnected";

const websocketBaseUrl = import.meta.env.VITE_WS_BASE_URL ?? "";

function buildWebSocketUrl() {
  if (websocketBaseUrl) {
    return `${websocketBaseUrl}/ws/alerts`;
  }

  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/ws/alerts`;
}

function isValidAlertRecord(data: unknown): data is AlertRecord {
  if (!data || typeof data !== "object") return false;
  const record = data as Record<string, unknown>;
  return typeof record.fingerprint === "string" && typeof record.status === "string";
}

export function useAlertsWebSocket(onAlert: (alert: AlertRecord) => void) {
  const connectionState = ref<ConnectionState>("disconnected");
  let socket: WebSocket | null = null;
  let reconnectTimer: number | null = null;
  let manuallyClosed = false;
  let reconnectDelay = 1000;
  const maxReconnectDelay = 30000;

  function clearReconnectTimer() {
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  function scheduleReconnect() {
    if (manuallyClosed) {
      return;
    }

    clearReconnectTimer();
    reconnectTimer = window.setTimeout(() => {
      connect();
    }, reconnectDelay);

    reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
  }

  function connect() {
    clearReconnectTimer();

    if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
      return;
    }

    manuallyClosed = false;
    reconnectDelay = 1000;
    connectionState.value = "connecting";
    socket = new WebSocket(buildWebSocketUrl());

    socket.onopen = () => {
      reconnectDelay = 1000;
      connectionState.value = "connected";
    };

    socket.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        if (isValidAlertRecord(payload)) {
          onAlert(payload);
        } else {
          console.error("Invalid alert record from websocket", payload);
        }
      } catch (error) {
        console.error("Failed to parse alert websocket message", error);
      }
    };

    socket.onclose = () => {
      connectionState.value = "disconnected";
      socket = null;
      scheduleReconnect();
    };

    socket.onerror = () => {
      // onclose will fire after onerror, handle reconnection there
    };
  }

  function disconnect() {
    manuallyClosed = true;
    clearReconnectTimer();

    if (socket) {
      socket.close();
      socket = null;
    }

    connectionState.value = "disconnected";
  }

  onBeforeUnmount(() => {
    disconnect();
  });

  return {
    connectionState,
    connect,
    disconnect,
  };
}
