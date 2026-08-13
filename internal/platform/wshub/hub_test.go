package wshub_test

import (
	"testing"

	"sre-kit/internal/platform/wshub"
)

type fakeSub struct{ received []wshub.Frame }

func (f *fakeSub) Send(frame wshub.Frame) { f.received = append(f.received, frame) }

func TestPublish_OnlyReachesSubscribedSourceID(t *testing.T) {
	hub := wshub.New()
	sub := &fakeSub{}
	hub.Register(sub)
	hub.Subscribe(sub, "src-1")

	hub.Publish(wshub.Frame{Type: "metric", SourceID: "src-1", Payload: "a"})
	hub.Publish(wshub.Frame{Type: "metric", SourceID: "src-2", Payload: "b"})

	if len(sub.received) != 1 || sub.received[0].SourceID != "src-1" {
		t.Fatalf("got %+v, want exactly one frame for src-1", sub.received)
	}
}

func TestUnsubscribe_StopsDelivery(t *testing.T) {
	hub := wshub.New()
	sub := &fakeSub{}
	hub.Register(sub)
	hub.Subscribe(sub, "src-1")
	hub.Unsubscribe(sub, "src-1")

	hub.Publish(wshub.Frame{Type: "metric", SourceID: "src-1", Payload: "a"})

	if len(sub.received) != 0 {
		t.Fatalf("got %+v, want no frames after unsubscribe", sub.received)
	}
}

func TestUnregister_StopsDelivery(t *testing.T) {
	hub := wshub.New()
	sub := &fakeSub{}
	hub.Register(sub)
	hub.Subscribe(sub, "src-1")
	hub.Unregister(sub)

	hub.Publish(wshub.Frame{Type: "metric", SourceID: "src-1", Payload: "a"})

	if len(sub.received) != 0 {
		t.Fatalf("got %+v, want no frames after unregister", sub.received)
	}
}
