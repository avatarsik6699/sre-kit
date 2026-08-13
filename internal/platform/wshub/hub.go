// Package wshub is the in-process pub/sub hub behind GET /api/stream (docs/SPEC.md §4/§5.2):
// connections subscribe to a set of source_ids, and Publish fans a frame out to every connection
// currently subscribed to that frame's source_id. Deferred item from
// docs/STACK.md § Backend Architecture ("internal/platform/wshub ... targeted for M4").
package wshub

import "sync"

// Frame is one pushed message: a Metric/Check/Event/Alert wire record plus its type discriminator,
// per docs/SPEC.md §4. The hub treats Payload as an opaque, already-JSON-marshalable value — it
// doesn't know or care about the telemetry domain types.
type Frame struct {
	Type     string `json:"type"`
	SourceID string `json:"source_id"`
	Payload  any    `json:"payload"`
}

// Subscriber receives frames published for the source_ids it's currently subscribed to. Hub
// dispatches on the goroutine that called Publish, so implementations must not block — the
// WS connection wrapper (internal/telemetry/interfaces/http) buffers via its own channel.
type Subscriber interface {
	Send(frame Frame)
}

// Hub is the in-process pub/sub registry. Zero value is not usable — construct with New.
type Hub struct {
	mu   sync.RWMutex
	subs map[Subscriber]map[string]struct{} // subscriber -> subscribed source_ids
}

// New constructs an empty Hub.
func New() *Hub {
	return &Hub{subs: make(map[Subscriber]map[string]struct{})}
}

// Register adds sub to the hub with no subscriptions yet. Call Unregister when the connection
// closes to avoid leaking the entry.
func (h *Hub) Register(sub Subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subs[sub] = make(map[string]struct{})
}

// Unregister removes sub and all of its subscriptions.
func (h *Hub) Unregister(sub Subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, sub)
}

// Subscribe adds sourceID to sub's subscription set. No-op if sub isn't registered.
func (h *Hub) Subscribe(sub Subscriber, sourceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.subs[sub]; ok {
		set[sourceID] = struct{}{}
	}
}

// Unsubscribe removes sourceID from sub's subscription set.
func (h *Hub) Unsubscribe(sub Subscriber, sourceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.subs[sub]; ok {
		delete(set, sourceID)
	}
}

// Publish fans frame out to every registered subscriber currently subscribed to frame.SourceID.
func (h *Hub) Publish(frame Frame) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for sub, set := range h.subs {
		if _, ok := set[frame.SourceID]; ok {
			sub.Send(frame)
		}
	}
}
