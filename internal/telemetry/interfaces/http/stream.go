// Package http — GET /api/stream: the live WS push feed of new Metric/Check/Event frames,
// filtered by subscribed source_ids, per docs/SPEC.md §4/§5.2. Session-gating is applied globally
// by the composition root (see handlers.go's package doc); the WS upgrade itself re-checks the
// session cookie since RequireSession runs before the protocol switches to WS framing.
package http

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"sre-kit/internal/platform/wshub"
)

// writeTimeout bounds how long a single frame push may block a slow client before the connection
// is dropped — Publish fans out synchronously (docs/STACK.md smallest-complete-implementation
// call), so one stuck client must not stall the whole hub indefinitely.
const writeTimeout = 5 * time.Second

// wsSubscriber adapts one *websocket.Conn to wshub.Subscriber.
type wsSubscriber struct {
	conn *websocket.Conn
}

func (s *wsSubscriber) Send(frame wshub.Frame) {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	if err := wsjson.Write(ctx, s.conn, frame); err != nil {
		log.Printf("stream: write frame to subscriber: %v", err)
	}
}

// clientMessage is a subscribe/unsubscribe control message sent by the browser client, per
// docs/SPEC.md §5.2 (client subscribes by currently-visible source_ids).
type clientMessage struct {
	Action   string `json:"action"` // "subscribe" | "unsubscribe"
	SourceID string `json:"source_id"`
}

// stream godoc
// @Summary      Live stream
// @Description  WebSocket: subscribe/unsubscribe by source_id, receive live Metric/Check/Event frames
// @Tags         telemetry
// @Security     SessionCookie
// @Router       /api/stream [get]
func (h *Handlers) stream(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	sub := &wsSubscriber{conn: conn}
	h.hub.Register(sub)
	defer h.hub.Unregister(sub)

	ctx := context.Background()
	for {
		var msg clientMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return
		}
		switch msg.Action {
		case "subscribe":
			h.hub.Subscribe(sub, msg.SourceID)
		case "unsubscribe":
			h.hub.Unsubscribe(sub, msg.SourceID)
		}
	}
}
