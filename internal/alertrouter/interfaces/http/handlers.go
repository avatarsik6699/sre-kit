// Package http exposes the alert router's HTTP surface: GET /api/alerts,
// /api/alert-rules (GET/POST/PATCH/DELETE), /api/notification-channels (GET/POST/PATCH/DELETE),
// per docs/SPEC.md §4. Session-gating is applied globally by the composition root, not by this
// package directly (mirrors internal/sources/interfaces/http).
package http

import (
	"encoding/json"
	"net/http"

	"sre-kit/internal/alertrouter/application"
	"sre-kit/internal/alertrouter/domain"
	"sre-kit/internal/platform/apierror"
)

// Handlers exposes the alert router HTTP surface bound to a *application.Service.
type Handlers struct {
	service *application.Service
}

// NewHandlers wires Handlers to svc.
func NewHandlers(svc *application.Service) *Handlers {
	return &Handlers{service: svc}
}

// Register mounts every alert-router route on mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/alerts", h.listAlerts)

	mux.HandleFunc("GET /api/alert-rules", h.listRules)
	mux.HandleFunc("POST /api/alert-rules", h.createRule)
	mux.HandleFunc("PATCH /api/alert-rules/{id}", h.updateRule)
	mux.HandleFunc("DELETE /api/alert-rules/{id}", h.deleteRule)

	mux.HandleFunc("GET /api/notification-channels", h.listChannels)
	mux.HandleFunc("POST /api/notification-channels", h.createChannel)
	mux.HandleFunc("PATCH /api/notification-channels/{id}", h.updateChannel)
	mux.HandleFunc("DELETE /api/notification-channels/{id}", h.deleteChannel)
}

// --- Alerts ---

type alertResponse struct {
	ID         string  `json:"id"`
	SourceID   string  `json:"source_id"`
	RuleID     *string `json:"rule_id"`
	Severity   string  `json:"severity"`
	Message    string  `json:"message"`
	CreatedAt  string  `json:"created_at"`
	ResolvedAt *string `json:"resolved_at"`
}

func toAlertResponse(alert domain.Alert) alertResponse {
	resp := alertResponse{
		ID: alert.ID, SourceID: alert.SourceID, RuleID: alert.RuleID,
		Severity: alert.Severity, Message: alert.Message, CreatedAt: alert.CreatedAt.Format(http.TimeFormat),
	}
	if alert.ResolvedAt != nil {
		formatted := alert.ResolvedAt.Format(http.TimeFormat)
		resp.ResolvedAt = &formatted
	}
	return resp
}

// listAlerts godoc
// @Summary      List alerts
// @Description  List alerts, optionally filtered by status
// @Tags         alerts
// @Produce      json
// @Security     SessionCookie
// @Param        status  query  string  false  "active | resolved"
// @Success      200  {array}   http.alertResponse
// @Failure      401  {object}  map[string]string
// @Router       /api/alerts [get]
func (h *Handlers) listAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.service.ListAlerts(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		apierror.Write(w, err)
		return
	}
	responses := make([]alertResponse, 0, len(alerts))
	for _, alert := range alerts {
		responses = append(responses, toAlertResponse(alert))
	}
	writeJSON(w, http.StatusOK, responses)
}

// --- Alert rules ---

type alertRuleResponse struct {
	ID              string `json:"id"`
	SourceID        string `json:"source_id"`
	TargetName      string `json:"target_name"`
	Condition       string `json:"condition"`
	Threshold       string `json:"threshold"`
	DebounceSeconds int    `json:"debounce_seconds"`
	NotifyChannelID string `json:"notify_channel_id"`
	Enabled         bool   `json:"enabled"`
}

func toRuleResponse(rule domain.AlertRule) alertRuleResponse {
	return alertRuleResponse{
		ID: rule.ID, SourceID: rule.SourceID, TargetName: rule.TargetName, Condition: rule.Condition,
		Threshold: rule.Threshold, DebounceSeconds: rule.DebounceSeconds, NotifyChannelID: rule.NotifyChannelID, Enabled: rule.Enabled,
	}
}

// listRules godoc
// @Summary      List alert rules
// @Description  List alert rules, optionally filtered by source
// @Tags         alert-rules
// @Produce      json
// @Security     SessionCookie
// @Param        source  query  string  false  "source ID filter"
// @Success      200  {array}   http.alertRuleResponse
// @Failure      401  {object}  map[string]string
// @Router       /api/alert-rules [get]
func (h *Handlers) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.ListRules(r.Context(), r.URL.Query().Get("source"))
	if err != nil {
		apierror.Write(w, err)
		return
	}
	responses := make([]alertRuleResponse, 0, len(rules))
	for _, rule := range rules {
		responses = append(responses, toRuleResponse(rule))
	}
	writeJSON(w, http.StatusOK, responses)
}

type createRuleRequest struct {
	SourceID        string `json:"source_id"`
	TargetName      string `json:"target_name"`
	Condition       string `json:"condition"`
	Threshold       string `json:"threshold"`
	DebounceSeconds int    `json:"debounce_seconds"`
	NotifyChannelID string `json:"notify_channel_id"`
}

// createRule godoc
// @Summary      Create an alert rule
// @Tags         alert-rules
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Param        rule  body      http.createRuleRequest  true  "rule fields"
// @Success      201   {object}  http.alertRuleResponse
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Router       /api/alert-rules [post]
func (h *Handlers) createRule(w http.ResponseWriter, r *http.Request) {
	var req createRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}
	rule, err := h.service.CreateRule(r.Context(), req.SourceID, req.TargetName, req.Condition, req.Threshold, req.DebounceSeconds, req.NotifyChannelID)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRuleResponse(rule))
}

type updateRuleRequest struct {
	Condition       *string `json:"condition"`
	Threshold       *string `json:"threshold"`
	DebounceSeconds *int    `json:"debounce_seconds"`
	NotifyChannelID *string `json:"notify_channel_id"`
	Enabled         *bool   `json:"enabled"`
}

// updateRule godoc
// @Summary      Update an alert rule
// @Tags         alert-rules
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Param        id    path      string                  true  "rule ID"
// @Param        rule  body      http.updateRuleRequest  true  "fields to patch"
// @Success      200   {object}  http.alertRuleResponse
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Router       /api/alert-rules/{id} [patch]
func (h *Handlers) updateRule(w http.ResponseWriter, r *http.Request) {
	var req updateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}
	rule, err := h.service.UpdateRule(r.Context(), r.PathValue("id"), req.Condition, req.Threshold, req.DebounceSeconds, req.NotifyChannelID, req.Enabled)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toRuleResponse(rule))
}

// deleteRule godoc
// @Summary      Delete an alert rule
// @Tags         alert-rules
// @Security     SessionCookie
// @Param        id  path  string  true  "rule ID"
// @Success      204
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /api/alert-rules/{id} [delete]
func (h *Handlers) deleteRule(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteRule(r.Context(), r.PathValue("id")); err != nil {
		apierror.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Notification channels ---

type notificationChannelResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	ChatID  string `json:"chat_id"`
	Enabled bool   `json:"enabled"`
}

func toChannelResponse(channel domain.NotificationChannel) notificationChannelResponse {
	var cfg struct {
		ChatID string `json:"chat_id"`
	}
	_ = json.Unmarshal([]byte(channel.ConfigJSON), &cfg)
	return notificationChannelResponse{ID: channel.ID, Type: channel.Type, ChatID: cfg.ChatID, Enabled: channel.Enabled}
}

// listChannels godoc
// @Summary      List notification channels
// @Description  List configured channels (never returns the credential value)
// @Tags         notification-channels
// @Produce      json
// @Security     SessionCookie
// @Success      200  {array}   http.notificationChannelResponse
// @Failure      401  {object}  map[string]string
// @Router       /api/notification-channels [get]
func (h *Handlers) listChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.service.ListChannels(r.Context())
	if err != nil {
		apierror.Write(w, err)
		return
	}
	responses := make([]notificationChannelResponse, 0, len(channels))
	for _, channel := range channels {
		responses = append(responses, toChannelResponse(channel))
	}
	writeJSON(w, http.StatusOK, responses)
}

type createChannelRequest struct {
	Type     string `json:"type"`
	ChatID   string `json:"chat_id"`
	BotToken string `json:"bot_token"`
}

// createChannel godoc
// @Summary      Create a notification channel
// @Description  type must be "telegram" in v1; bot_token is stored via the secrets store, never persisted or returned in plaintext
// @Tags         notification-channels
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Param        channel  body      http.createChannelRequest  true  "type, chat_id, bot_token"
// @Success      201      {object}  http.notificationChannelResponse
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Router       /api/notification-channels [post]
func (h *Handlers) createChannel(w http.ResponseWriter, r *http.Request) {
	var req createChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}
	channel, err := h.service.CreateChannel(r.Context(), req.Type, req.ChatID, req.BotToken)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toChannelResponse(channel))
}

type updateChannelRequest struct {
	ChatID   *string `json:"chat_id"`
	BotToken *string `json:"bot_token"`
	Enabled  *bool   `json:"enabled"`
}

// updateChannel godoc
// @Summary      Update a notification channel
// @Description  Update chat_id / enable / disable / rotate bot_token
// @Tags         notification-channels
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Param        id       path      string                     true  "channel ID"
// @Param        channel  body      http.updateChannelRequest  true  "fields to patch"
// @Success      200      {object}  http.notificationChannelResponse
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Failure      404      {object}  map[string]string
// @Router       /api/notification-channels/{id} [patch]
func (h *Handlers) updateChannel(w http.ResponseWriter, r *http.Request) {
	var req updateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}
	channel, err := h.service.UpdateChannel(r.Context(), r.PathValue("id"), req.ChatID, req.BotToken, req.Enabled)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toChannelResponse(channel))
}

// deleteChannel godoc
// @Summary      Delete a notification channel
// @Description  Blocked (409) if an enabled alert rule still references it
// @Tags         notification-channels
// @Security     SessionCookie
// @Param        id  path  string  true  "channel ID"
// @Success      204
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Router       /api/notification-channels/{id} [delete]
func (h *Handlers) deleteChannel(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteChannel(r.Context(), r.PathValue("id")); err != nil {
		apierror.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
