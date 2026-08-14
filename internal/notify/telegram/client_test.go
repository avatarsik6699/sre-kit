package telegram_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sre-kit/internal/notify/telegram"
)

func TestClient_Send(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := telegram.NewClient().WithBaseURL(server.URL)
	if err := client.Send(context.Background(), "tok123", "chat1", "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotPath != "/bottok123/sendMessage" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["chat_id"] != "chat1" || gotBody["text"] != "hello" {
		t.Fatalf("body = %+v", gotBody)
	}
}

func TestClient_Send_APIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer server.Close()

	client := telegram.NewClient().WithBaseURL(server.URL)
	err := client.Send(context.Background(), "tok123", "bad-chat", "hello")
	if err == nil {
		t.Fatal("expected error")
	}
}
