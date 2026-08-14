// Single GET /api/stream WebSocket connection, pub/sub by currently-visible source_id
// (docs/SPEC.md §5.2). Consumers register via subscribe(); onResync fires once immediately (the
// initial snapshot) and again on every reconnect, so callers can drive their own REST snapshot
// fetch there and only rely on onFrame for deltas. A relative WebSocket URL resolves against the
// page's own origin, so this module never needs `window`/`location` (docs/FRONTEND_CONVENTIONS.md
// §6 no-restricted-globals).

export type StreamMetricPayload = {
  source_id: string;
  name: string;
  ts: string;
  value: number;
  labels: string;
};

export type StreamCheckPayload = {
  source_id: string;
  name: string;
  ts: string;
  status: string;
  meta: string;
};

export type StreamEventPayload = {
  source_id: string;
  ts: string;
  level: string;
  message: string;
  labels: string;
};

export type StreamAlertPayload = {
  id: string;
  source_id: string;
  rule_id?: string;
  severity: string;
  message: string;
  created_at: string;
  resolved_at?: string;
};

export type StreamFrame =
  | { type: "metric"; source_id: string; payload: StreamMetricPayload }
  | { type: "check"; source_id: string; payload: StreamCheckPayload }
  | { type: "event"; source_id: string; payload: StreamEventPayload }
  | { type: "alert"; source_id: string; payload: StreamAlertPayload };

type Subscription = {
  onFrame: (frame: StreamFrame) => void;
  onResync: () => void;
};

const RECONNECT_DELAY_MS = 2000;

class WsStreamStore {
  private socket: WebSocket | null = null;
  private subscriptions = new Map<string, Set<Subscription>>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  /** Subscribes to frames for sourceId. Returns an unsubscribe function. */
  subscribe(sourceId: string, sub: Subscription): () => void {
    let subsForSource = this.subscriptions.get(sourceId);
    if (!subsForSource) {
      subsForSource = new Set();
      this.subscriptions.set(sourceId, subsForSource);
    }
    const isFirstForSource = subsForSource.size === 0;
    subsForSource.add(sub);

    this.ensureConnected();
    if (isFirstForSource) {
      this.send("subscribe", sourceId);
    }
    // Every new subscriber gets its own initial snapshot signal, independent of socket state —
    // the REST snapshot fetch it triggers doesn't need the WS connection to be open yet.
    sub.onResync();

    return () => {
      subsForSource?.delete(sub);
      if (subsForSource && subsForSource.size === 0) {
        this.subscriptions.delete(sourceId);
        this.send("unsubscribe", sourceId);
      }
    };
  }

  private ensureConnected(): void {
    if (
      this.socket &&
      (this.socket.readyState === WebSocket.OPEN ||
        this.socket.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }

    const socket = new WebSocket("/api/stream");
    this.socket = socket;

    socket.addEventListener("open", () => {
      for (const sourceId of this.subscriptions.keys()) {
        this.send("subscribe", sourceId);
      }
      for (const subsForSource of this.subscriptions.values()) {
        for (const sub of subsForSource) {
          sub.onResync();
        }
      }
    });

    socket.addEventListener("message", (event: MessageEvent<string>) => {
      let frame: StreamFrame;
      try {
        frame = JSON.parse(event.data) as StreamFrame;
      } catch {
        return;
      }
      const subsForSource = this.subscriptions.get(frame.source_id);
      if (!subsForSource) {
        return;
      }
      for (const sub of subsForSource) {
        sub.onFrame(frame);
      }
    });

    socket.addEventListener("close", () => {
      if (this.socket === socket) {
        this.socket = null;
      }
      if (this.subscriptions.size > 0) {
        this.scheduleReconnect();
      }
    });

    socket.addEventListener("error", () => {
      socket.close();
    });
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) {
      return;
    }
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (this.subscriptions.size > 0) {
        this.ensureConnected();
      }
    }, RECONNECT_DELAY_MS);
  }

  private send(action: "subscribe" | "unsubscribe", sourceId: string): void {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ action, source_id: sourceId }));
    }
  }
}

/** Module-level singleton — the one real cross-route store this app needs (docs/SPEC.md §5.2). */
export const wsStreamStore = new WsStreamStore();
