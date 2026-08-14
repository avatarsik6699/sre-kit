// Package telegram implements alertrouter/application's Notifier port against the Telegram Bot
// API, per docs/SPEC.md §1.3's single-channel v1 notification target.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// apiBaseURL is the Telegram Bot API base — https://core.telegram.org/bots/api#sendmessage.
// Overridable in tests via WithBaseURL.
const apiBaseURL = "https://api.telegram.org"

// Client sends messages via the Telegram Bot API. Structurally satisfies alertrouter/application's
// Notifier port (Send(ctx, botToken, chatID, message string) error) without either package
// importing the other.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient constructs a Client with a sane request timeout.
func NewClient() *Client {
	return &Client{httpClient: &http.Client{}, baseURL: apiBaseURL}
}

// WithBaseURL overrides the Telegram API base URL — test-only hook so Send can be exercised
// against an httptest.Server instead of the real Telegram API.
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = baseURL
	return c
}

type sendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type sendMessageResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

// Send delivers message to chatID via the bot identified by botToken. Returns an error on any
// non-2xx response or a Telegram-reported failure (ok: false).
func (c *Client) Send(ctx context.Context, botToken, chatID, message string) error {
	body, err := json.Marshal(sendMessageRequest{ChatID: chatID, Text: message})
	if err != nil {
		return fmt.Errorf("telegram: encode request: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send message: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("telegram: read response: %w", err)
	}
	var parsed sendMessageResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("telegram: decode response (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode >= 300 || !parsed.OK {
		return fmt.Errorf("telegram: send message failed (status %d): %s", resp.StatusCode, parsed.Description)
	}
	return nil
}
