import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { wsStreamStore } from "~/shared/lib/ws-stream-store";

// Node's global WebSocket (undici) requires an absolute URL — it doesn't resolve relative URLs
// against a page origin the way a real browser does, unlike jsdom's DOM APIs. The store
// deliberately uses a relative URL ("/api/stream", see its own comment) since that's valid and
// origin-safe in the real browser it actually runs in; this fake WebSocket stands in for the
// test environment's gap, not a change to the store's real behavior.
class FakeWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  readyState = FakeWebSocket.CONNECTING;
  private listeners = new Map<string, Set<(event: unknown) => void>>();

  addEventListener(type: string, listener: (event: unknown) => void): void {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, new Set());
    }
    this.listeners.get(type)?.add(listener);
  }

  send(): void {}
  close(): void {}

  dispatch(type: string, event: unknown = {}): void {
    for (const listener of this.listeners.get(type) ?? []) {
      listener(event);
    }
  }
}

let originalWebSocket: typeof WebSocket;

beforeEach(() => {
  originalWebSocket = globalThis.WebSocket;
  globalThis.WebSocket = FakeWebSocket as unknown as typeof WebSocket;
});

afterEach(() => {
  globalThis.WebSocket = originalWebSocket;
});

describe("wsStreamStore", () => {
  it("fires onResync immediately on subscribe", () => {
    const onResync = vi.fn();
    const unsubscribe = wsStreamStore.subscribe("src-test-1", {
      onFrame: vi.fn(),
      onResync,
    });
    expect(onResync).toHaveBeenCalledTimes(1);
    unsubscribe();
  });

  it("stops firing after unsubscribe", () => {
    const onFrame = vi.fn();
    const unsubscribe = wsStreamStore.subscribe("src-test-2", {
      onFrame,
      onResync: vi.fn(),
    });
    unsubscribe();
    expect(() => unsubscribe()).not.toThrow();
  });
});
