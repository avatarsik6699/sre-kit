// Package application holds the alert router's use-cases: evaluate incoming Metric/Check data and
// source connectivity status against AlertRule/system thresholds, drive the firing->resolved
// lifecycle (docs/SPEC.md §6) with debounce/flap protection, notify via a NotificationChannel, and
// the CRUD use-cases behind /api/alert-rules and /api/notification-channels.
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"sre-kit/internal/alertrouter/domain"
	"sre-kit/internal/platform/apierror"
)

// unreachableConsecutiveThreshold / unreachableContinuousWindow implement docs/SPEC.md §6's
// unreachable debounce: "3 consecutive failures or 5 minutes continuous", whichever comes first.
const (
	unreachableConsecutiveThreshold = 3
	unreachableContinuousWindow     = 5 * time.Minute
)

// sourceStatus mirrors internal/sources/domain's Status* constants as plain strings, so this
// package doesn't import internal/sources (ports-over-direct-imports, docs/STACK.md).
const (
	sourceStatusOK          = "ok"
	sourceStatusUnreachable = "unreachable"
	sourceStatusError       = "error"
)

// pendingRule tracks an AlertRule whose condition has been continuously true since firstTrueAt,
// waiting out its debounce window before the router opens an Alert.
type pendingRule struct {
	firstTrueAt time.Time
}

// sourceStatusState tracks one source's connectivity debounce state for EvaluateSourceStatus.
type sourceStatusState struct {
	unreachableSince       time.Time
	consecutiveUnreachable int
}

// Service implements alert evaluation and the AlertRule/NotificationChannel/Alert use-cases.
type Service struct {
	alerts   domain.AlertRepository
	rules    domain.AlertRuleRepository
	channels domain.NotificationChannelRepository
	secrets  SecretStore

	notifier  Notifier
	publisher Publisher

	now func() time.Time

	mu             sync.Mutex
	pendingRules   map[string]pendingRule       // ruleID -> pending state
	sourceStatuses map[string]sourceStatusState // sourceID -> connectivity debounce state
}

// Option configures optional Service dependencies.
type Option func(*Service)

// WithNotifier wires notifier as the Service's notification-delivery target.
func WithNotifier(notifier Notifier) Option {
	return func(s *Service) { s.notifier = notifier }
}

// WithPublisher wires pub as the Service's live-stream fan-out target.
func WithPublisher(pub Publisher) Option {
	return func(s *Service) { s.publisher = pub }
}

// NewService wires a Service to its repositories and secret store, plus any Options.
func NewService(alerts domain.AlertRepository, rules domain.AlertRuleRepository, channels domain.NotificationChannelRepository, secrets SecretStore, opts ...Option) *Service {
	s := &Service{
		alerts:         alerts,
		rules:          rules,
		channels:       channels,
		secrets:        secrets,
		now:            time.Now,
		pendingRules:   make(map[string]pendingRule),
		sourceStatuses: make(map[string]sourceStatusState),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// --- Evaluation ---

// EvaluateMetric checks value against every enabled AlertRule on sourceID targeting name.
func (s *Service) EvaluateMetric(ctx context.Context, sourceID, name string, value float64) error {
	rules, err := s.rules.List(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("alertrouter: evaluate metric: list rules: %w", err)
	}
	for _, rule := range rules {
		if !rule.Enabled || rule.TargetName != name || rule.Condition == domain.ConditionStatusIs {
			continue
		}
		matched, err := domain.EvaluateMetricCondition(rule.Condition, rule.Threshold, value)
		if err != nil {
			return fmt.Errorf("alertrouter: evaluate metric rule %s: %w", rule.ID, err)
		}
		if err := s.applyRuleResult(ctx, rule, matched, fmt.Sprintf("%s %s %s (current: %.2f)", name, rule.Condition, rule.Threshold, value)); err != nil {
			return err
		}
	}
	return nil
}

// EvaluateCheck checks status against every enabled AlertRule on sourceID targeting name.
func (s *Service) EvaluateCheck(ctx context.Context, sourceID, name, status string) error {
	rules, err := s.rules.List(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("alertrouter: evaluate check: list rules: %w", err)
	}
	for _, rule := range rules {
		if !rule.Enabled || rule.TargetName != name || rule.Condition != domain.ConditionStatusIs {
			continue
		}
		matched, err := domain.EvaluateCheckCondition(rule.Condition, rule.Threshold, status)
		if err != nil {
			return fmt.Errorf("alertrouter: evaluate check rule %s: %w", rule.ID, err)
		}
		if err := s.applyRuleResult(ctx, rule, matched, fmt.Sprintf("%s status is %s (threshold: %s)", name, status, rule.Threshold)); err != nil {
			return err
		}
	}
	return nil
}

// applyRuleResult drives the debounce state machine for one rule evaluation and opens/resolves
// its Alert as needed.
func (s *Service) applyRuleResult(ctx context.Context, rule domain.AlertRule, matched bool, message string) error {
	now := s.now()

	if !matched {
		s.mu.Lock()
		delete(s.pendingRules, rule.ID)
		s.mu.Unlock()
		return s.resolveByRule(ctx, rule.ID)
	}

	s.mu.Lock()
	pending, ok := s.pendingRules[rule.ID]
	if !ok {
		pending = pendingRule{firstTrueAt: now}
		s.pendingRules[rule.ID] = pending
	}
	elapsed := now.Sub(pending.firstTrueAt)
	s.mu.Unlock()

	if elapsed < time.Duration(rule.DebounceSeconds)*time.Second {
		return nil
	}

	_, open, err := s.alerts.FindOpenByRule(ctx, rule.ID)
	if err != nil {
		return fmt.Errorf("alertrouter: check open alert for rule %s: %w", rule.ID, err)
	}
	if open {
		return nil
	}
	ruleID := rule.ID
	alert := domain.Alert{
		ID:        newID(),
		SourceID:  rule.SourceID,
		RuleID:    &ruleID,
		Severity:  domain.SeverityCritical,
		Message:   message,
		CreatedAt: now,
	}
	return s.fire(ctx, alert, rule.NotifyChannelID)
}

func (s *Service) resolveByRule(ctx context.Context, ruleID string) error {
	alert, open, err := s.alerts.FindOpenByRule(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("alertrouter: find open alert for rule %s: %w", ruleID, err)
	}
	if !open {
		return nil
	}
	rule, err := s.rules.Get(ctx, ruleID)
	notifyChannelID := ""
	if err == nil {
		notifyChannelID = rule.NotifyChannelID
	}
	return s.resolve(ctx, alert, notifyChannelID)
}

// EvaluateSourceStatus drives the built-in (rule-less) connectivity alert for sourceID, per
// docs/SPEC.md §6: "unreachable" debounces (3 consecutive failures or 5 min continuous),
// "error" fires immediately, "ok" resolves any open connectivity alert. This has no NotifyChannel
// of its own — v1 has no UI to configure one for a system-level alert, so it fires/resolves
// silently on the WS stream (still visible on the Dashboard/Alerts rail) without a Telegram push.
// See docs/changes/archive/05-alert-router-telegram.md Implementation Notes for the scope decision this
// reflects (adapterengine, which is where "unreachable" would actually be detected, is out of this
// change's Files list).
func (s *Service) EvaluateSourceStatus(ctx context.Context, sourceID, status string) error {
	now := s.now()

	switch status {
	case sourceStatusOK:
		s.mu.Lock()
		delete(s.sourceStatuses, sourceID)
		s.mu.Unlock()
		alert, open, err := s.alerts.FindOpenSystemAlert(ctx, sourceID)
		if err != nil {
			return fmt.Errorf("alertrouter: find open system alert: %w", err)
		}
		if !open {
			return nil
		}
		return s.resolve(ctx, alert, "")

	case sourceStatusError:
		s.mu.Lock()
		delete(s.sourceStatuses, sourceID)
		s.mu.Unlock()
		return s.fireSystemAlertIfNotOpen(ctx, sourceID, domain.SeverityCritical, "source reported an error")

	case sourceStatusUnreachable:
		s.mu.Lock()
		state, ok := s.sourceStatuses[sourceID]
		if !ok {
			state = sourceStatusState{unreachableSince: now}
		}
		state.consecutiveUnreachable++
		s.sourceStatuses[sourceID] = state
		elapsed := now.Sub(state.unreachableSince)
		consecutive := state.consecutiveUnreachable
		s.mu.Unlock()

		if consecutive < unreachableConsecutiveThreshold && elapsed < unreachableContinuousWindow {
			return nil
		}
		return s.fireSystemAlertIfNotOpen(ctx, sourceID, domain.SeverityWarning, "source is unreachable")

	default:
		return nil
	}
}

func (s *Service) fireSystemAlertIfNotOpen(ctx context.Context, sourceID, severity, message string) error {
	_, open, err := s.alerts.FindOpenSystemAlert(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("alertrouter: check open system alert: %w", err)
	}
	if open {
		return nil
	}
	alert := domain.Alert{
		ID:        newID(),
		SourceID:  sourceID,
		Severity:  severity,
		Message:   message,
		CreatedAt: s.now(),
	}
	return s.fire(ctx, alert, "")
}

// fire persists alert as newly opened, publishes it, and notifies notifyChannelID (if any).
func (s *Service) fire(ctx context.Context, alert domain.Alert, notifyChannelID string) error {
	if err := s.alerts.Create(ctx, alert); err != nil {
		return fmt.Errorf("alertrouter: create alert: %w", err)
	}
	s.publish(alert)
	s.notify(ctx, notifyChannelID, alert, true)
	return nil
}

// resolve stamps alert resolved, publishes the update, and notifies notifyChannelID (if any).
func (s *Service) resolve(ctx context.Context, alert domain.Alert, notifyChannelID string) error {
	resolvedAt := s.now()
	if err := s.alerts.Resolve(ctx, alert.ID, resolvedAt); err != nil {
		return fmt.Errorf("alertrouter: resolve alert: %w", err)
	}
	alert.ResolvedAt = &resolvedAt
	s.publish(alert)
	s.notify(ctx, notifyChannelID, alert, false)
	return nil
}

func (s *Service) publish(alert domain.Alert) {
	if s.publisher == nil {
		return
	}
	s.publisher.Publish(Frame{Type: "alert", SourceID: alert.SourceID, Payload: alertFramePayload(alert)})
}

func alertFramePayload(alert domain.Alert) map[string]any {
	payload := map[string]any{
		"id": alert.ID, "source_id": alert.SourceID, "severity": alert.Severity,
		"message": alert.Message, "created_at": alert.CreatedAt.Format(time.RFC3339),
	}
	if alert.RuleID != nil {
		payload["rule_id"] = *alert.RuleID
	}
	if alert.ResolvedAt != nil {
		payload["resolved_at"] = alert.ResolvedAt.Format(time.RFC3339)
	}
	return payload
}

// notify delivers alert's message via notifyChannelID, best-effort: a delivery failure (bad
// token, Telegram outage, channel disabled) must never fail an already-persisted alert
// transition, mirroring telemetry/application.Service.markSeen's same-shaped drop-the-error
// pattern.
func (s *Service) notify(ctx context.Context, notifyChannelID string, alert domain.Alert, fired bool) {
	if s.notifier == nil || notifyChannelID == "" {
		return
	}
	channel, err := s.channels.Get(ctx, notifyChannelID)
	if err != nil || !channel.Enabled || channel.Type != domain.ChannelTypeTelegram {
		return
	}
	var cfg struct {
		ChatID string `json:"chat_id"`
	}
	if err := json.Unmarshal([]byte(channel.ConfigJSON), &cfg); err != nil || cfg.ChatID == "" {
		return
	}
	token, err := s.secrets.Get(channel.SecretRef)
	if err != nil {
		return
	}
	message := formatMessage(alert, fired)
	_ = s.notifier.Send(ctx, token, cfg.ChatID, message)
}

func formatMessage(alert domain.Alert, fired bool) string {
	if fired {
		return fmt.Sprintf("🔴 ALERT [%s] source %s: %s", alert.Severity, alert.SourceID, alert.Message)
	}
	return fmt.Sprintf("✅ RESOLVED source %s: %s", alert.SourceID, alert.Message)
}

// --- Alert queries ---

// ListAlerts returns alerts filtered by status ("", "active", "resolved").
func (s *Service) ListAlerts(ctx context.Context, status string) ([]domain.Alert, error) {
	alerts, err := s.alerts.List(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("alertrouter: list alerts: %w", err)
	}
	return alerts, nil
}

// --- AlertRule CRUD ---

// CreateRule validates and persists a new AlertRule.
func (s *Service) CreateRule(ctx context.Context, sourceID, targetName, condition, threshold string, debounceSeconds int, notifyChannelID string) (domain.AlertRule, error) {
	rule := domain.AlertRule{
		ID: newID(), SourceID: sourceID, TargetName: targetName, Condition: condition,
		Threshold: threshold, DebounceSeconds: debounceSeconds, NotifyChannelID: notifyChannelID, Enabled: true,
	}
	if err := s.validateRule(ctx, rule); err != nil {
		return domain.AlertRule{}, err
	}
	if err := s.rules.Create(ctx, rule); err != nil {
		return domain.AlertRule{}, fmt.Errorf("alertrouter: create rule: %w", err)
	}
	return rule, nil
}

// UpdateRule patches an existing rule; nil fields leave the current value unchanged.
func (s *Service) UpdateRule(ctx context.Context, id string, condition, threshold *string, debounceSeconds *int, notifyChannelID *string, enabled *bool) (domain.AlertRule, error) {
	rule, err := s.rules.Get(ctx, id)
	if err != nil {
		return domain.AlertRule{}, err
	}
	if condition != nil {
		rule.Condition = *condition
	}
	if threshold != nil {
		rule.Threshold = *threshold
	}
	if debounceSeconds != nil {
		rule.DebounceSeconds = *debounceSeconds
	}
	if notifyChannelID != nil {
		rule.NotifyChannelID = *notifyChannelID
	}
	if enabled != nil {
		rule.Enabled = *enabled
	}
	if err := s.validateRule(ctx, rule); err != nil {
		return domain.AlertRule{}, err
	}
	if err := s.rules.Update(ctx, rule); err != nil {
		return domain.AlertRule{}, fmt.Errorf("alertrouter: update rule: %w", err)
	}
	return rule, nil
}

// DeleteRule removes a rule permanently. Its debounce state (if pending) is dropped; any
// already-open Alert it created is left as historical record.
func (s *Service) DeleteRule(ctx context.Context, id string) error {
	if err := s.rules.Delete(ctx, id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.pendingRules, id)
	s.mu.Unlock()
	return nil
}

// ListRules returns rules, optionally filtered to one sourceID ("" = all).
func (s *Service) ListRules(ctx context.Context, sourceID string) ([]domain.AlertRule, error) {
	rules, err := s.rules.List(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("alertrouter: list rules: %w", err)
	}
	return rules, nil
}

func (s *Service) validateRule(ctx context.Context, rule domain.AlertRule) error {
	if rule.SourceID == "" {
		return apierror.Invalid("source_id is required")
	}
	if rule.TargetName == "" {
		return apierror.Invalid("target_name is required")
	}
	switch rule.Condition {
	case domain.ConditionGreaterThan, domain.ConditionLessThan, domain.ConditionEquals, domain.ConditionStatusIs:
	default:
		return apierror.Invalid("condition must be one of >, <, =, status_is")
	}
	if rule.Threshold == "" {
		return apierror.Invalid("threshold is required")
	}
	if rule.DebounceSeconds < 0 {
		return apierror.Invalid("debounce_seconds must not be negative")
	}
	if rule.NotifyChannelID != "" {
		if _, err := s.channels.Get(ctx, rule.NotifyChannelID); err != nil {
			return apierror.Invalid("notify_channel_id does not reference an existing channel")
		}
	}
	return nil
}

// --- NotificationChannel CRUD ---

// CreateChannel validates and persists a new Telegram NotificationChannel, storing botToken via
// the secrets store and keeping only its ref in ConfigJSON's sibling SecretRef field.
func (s *Service) CreateChannel(ctx context.Context, channelType, chatID, botToken string) (domain.NotificationChannel, error) {
	if channelType != domain.ChannelTypeTelegram {
		return domain.NotificationChannel{}, apierror.Invalid("type must be \"telegram\"")
	}
	if chatID == "" {
		return domain.NotificationChannel{}, apierror.Invalid("chat_id is required")
	}
	if botToken == "" {
		return domain.NotificationChannel{}, apierror.Invalid("bot_token is required")
	}
	ref, err := s.secrets.Put(botToken)
	if err != nil {
		return domain.NotificationChannel{}, fmt.Errorf("alertrouter: store bot token: %w", err)
	}
	configJSON, err := json.Marshal(map[string]string{"chat_id": chatID})
	if err != nil {
		return domain.NotificationChannel{}, fmt.Errorf("alertrouter: encode config: %w", err)
	}
	channel := domain.NotificationChannel{ID: newID(), Type: channelType, ConfigJSON: string(configJSON), SecretRef: ref, Enabled: true}
	if err := s.channels.Create(ctx, channel); err != nil {
		return domain.NotificationChannel{}, fmt.Errorf("alertrouter: create channel: %w", err)
	}
	return channel, nil
}

// UpdateChannel patches an existing channel's chat_id, enabled state, and/or rotates its bot
// token; nil fields leave the current value unchanged.
func (s *Service) UpdateChannel(ctx context.Context, id string, chatID, botToken *string, enabled *bool) (domain.NotificationChannel, error) {
	channel, err := s.channels.Get(ctx, id)
	if err != nil {
		return domain.NotificationChannel{}, err
	}
	if chatID != nil {
		configJSON, err := json.Marshal(map[string]string{"chat_id": *chatID})
		if err != nil {
			return domain.NotificationChannel{}, fmt.Errorf("alertrouter: encode config: %w", err)
		}
		channel.ConfigJSON = string(configJSON)
	}
	if botToken != nil && *botToken != "" {
		newRef, err := s.secrets.Put(*botToken)
		if err != nil {
			return domain.NotificationChannel{}, fmt.Errorf("alertrouter: store rotated bot token: %w", err)
		}
		oldRef := channel.SecretRef
		channel.SecretRef = newRef
		_ = s.secrets.Delete(oldRef)
	}
	if enabled != nil {
		channel.Enabled = *enabled
	}
	if err := s.channels.Update(ctx, channel); err != nil {
		return domain.NotificationChannel{}, fmt.Errorf("alertrouter: update channel: %w", err)
	}
	return channel, nil
}

// DeleteChannel removes a channel permanently, refusing while an enabled AlertRule still
// references it.
func (s *Service) DeleteChannel(ctx context.Context, id string) error {
	channel, err := s.channels.Get(ctx, id)
	if err != nil {
		return err
	}
	inUse, err := s.rules.ListEnabledByChannel(ctx, id)
	if err != nil {
		return fmt.Errorf("alertrouter: check channel in use: %w", err)
	}
	if len(inUse) > 0 {
		return domain.ErrChannelInUse
	}
	if err := s.channels.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.secrets.Delete(channel.SecretRef)
	return nil
}

// ListChannels returns every configured channel.
func (s *Service) ListChannels(ctx context.Context) ([]domain.NotificationChannel, error) {
	channels, err := s.channels.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("alertrouter: list channels: %w", err)
	}
	return channels, nil
}
