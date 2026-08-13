package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"sre-kit/internal/platform/wshub"
	"sre-kit/internal/telemetry/application"
	telemetryhttp "sre-kit/internal/telemetry/interfaces/http"
)

func TestStream_DeliversOnlySubscribedSource(t *testing.T) {
	hub := wshub.New()
	svc := application.NewService(&fakeMetricRepo{}, &fakeCheckRepo{}, &fakeEventRepo{}, application.WithPublisher(hubPublisher{hub}))

	mux := http.NewServeMux()
	telemetryhttp.NewHandlers(svc, hub).Register(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):] + "/api/stream"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	if err := wsjson.Write(ctx, conn, map[string]string{"action": "subscribe", "source_id": "src-1"}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Give the server goroutine time to process the subscribe message before publishing.
	time.Sleep(50 * time.Millisecond)

	if err := svc.IngestMetric(context.Background(), "src-2", "cpu.usage_percent", time.Now(), 1, nil); err != nil {
		t.Fatalf("ingest src-2: %v", err)
	}
	if err := svc.IngestMetric(context.Background(), "src-1", "cpu.usage_percent", time.Now(), 2, nil); err != nil {
		t.Fatalf("ingest src-1: %v", err)
	}

	var frame wshub.Frame
	if err := wsjson.Read(ctx, conn, &frame); err != nil {
		t.Fatalf("read: %v", err)
	}
	if frame.SourceID != "src-1" || frame.Type != "metric" {
		t.Fatalf("got frame %+v, want the src-1 metric frame", frame)
	}
}

type hubPublisher struct{ hub *wshub.Hub }

func (p hubPublisher) Publish(frame application.Frame) {
	p.hub.Publish(wshub.Frame{Type: frame.Type, SourceID: frame.SourceID, Payload: frame.Payload})
}
