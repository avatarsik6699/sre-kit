// React hook wrapping shared/lib/ws-stream-store's subscribe() (docs/SPEC.md §5.2).
import { useEffect, useRef } from "react";
import { wsStreamStore, type StreamFrame } from "~/shared/lib/ws-stream-store";

/**
 * Subscribes to live frames for sourceId for the lifetime of the calling component. onFrame
 * receives each delta; onResync fires once on mount and again on every WS reconnect — callers use
 * it to trigger their own REST snapshot re-fetch (docs/SPEC.md §5.2's reconnect-then-resnapshot
 * behavior).
 */
export function useStreamSubscription(
  sourceId: string,
  onFrame: (frame: StreamFrame) => void,
  onResync: () => void,
): void {
  const onFrameRef = useRef(onFrame);
  const onResyncRef = useRef(onResync);

  useEffect(function syncStreamCallbackRefsFx() {
    onFrameRef.current = onFrame;
    onResyncRef.current = onResync;
  });

  useEffect(
    function subscribeToStreamFx() {
      if (sourceId === "") {
        return undefined;
      }
      return wsStreamStore.subscribe(sourceId, {
        onFrame: (frame) => onFrameRef.current(frame),
        onResync: () => onResyncRef.current(),
      });
    },
    [sourceId],
  );
}
