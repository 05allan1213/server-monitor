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

export function useAlertsWebSocket(onAlert: (alert: AlertRecord) => void) {
  const connectionState = ref<ConnectionState>("disconnected");
  let socket: WebSocket | null = null;
  let reconnectTimer: number | null = null;
  let manuallyClosed = false;

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
    }, 3000);
  }

  function connect() {
    clearReconnectTimer();

    if (socket && socket.readyState === WebSocket.OPEN) {
      return;
    }

    manuallyClosed = false;
    connectionState.value = "connecting";
    socket = new WebSocket(buildWebSocketUrl());

    socket.onopen = () => {
      connectionState.value = "connected";
    };

    socket.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data) as AlertRecord;
        onAlert(payload);
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
      connectionState.value = "disconnected";
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
