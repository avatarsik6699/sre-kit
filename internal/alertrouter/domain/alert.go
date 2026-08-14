// Package domain holds the Alert, AlertRule, and NotificationChannel entities, their repository
// ports, and the pure condition/debounce evaluation logic, per docs/SPEC.md §3/§6.
package domain

import (
	"context"
	"time"

	"sre-kit/internal/platform/apierror"
)

// Severity values for Alert.Severity.
const (
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Alert is one firing/resolved instance, derived by the router from an AlertRule (RuleID set) or
// generated directly from a source's connectivity status (RuleID nil — docs/SPEC.md §6's
// unreachable/error handling, no user-defined rule involved).
type Alert struct {
	ID         string
	SourceID   string
	RuleID     *string
	Severity   string
	Message    string
	CreatedAt  time.Time
	ResolvedAt *time.Time
}

// AlertRule matches a Metric/Check by name and condition and, when it fires, opens an Alert and
// notifies NotifyChannelID.
type AlertRule struct {
	ID              string
	SourceID        string
	TargetName      string
	Condition       string // ">" | "<" | "=" | "status_is"
	Threshold       string
	DebounceSeconds int
	NotifyChannelID string
	Enabled         bool
}

// Condition values AlertRule.Condition accepts, per docs/SPEC.md §3.
const (
	ConditionGreaterThan = ">"
	ConditionLessThan    = "<"
	ConditionEquals      = "="
	ConditionStatusIs    = "status_is"
)

// NotificationChannel is a configured notification target — "telegram" only in v1. The bot token
// (or equivalent credential) is never stored here; SecretRef points at it in the shared
// internal/platform/secrets store (docs/SPEC.md §3).
type NotificationChannel struct {
	ID         string
	Type       string // "telegram" only in v1
	ConfigJSON string // non-secret config, e.g. {"chat_id": "..."}
	SecretRef  string
	Enabled    bool
}

// ChannelTypeTelegram is the only NotificationChannel.Type accepted in v1.
const ChannelTypeTelegram = "telegram"

// ErrAlertNotFound is returned when a lookup by ID finds no matching alert.
var ErrAlertNotFound = apierror.NotFound("alert not found")

// ErrRuleNotFound is returned when a lookup by ID finds no matching alert rule.
var ErrRuleNotFound = apierror.NotFound("alert rule not found")

// ErrChannelNotFound is returned when a lookup by ID finds no matching notification channel.
var ErrChannelNotFound = apierror.NotFound("notification channel not found")

// ErrChannelInUse is returned when deleting a NotificationChannel still referenced by an enabled
// AlertRule.
var ErrChannelInUse = apierror.Conflict("notification channel is referenced by an enabled alert rule")

// AlertRepository is the persistence port for Alert.
type AlertRepository interface {
	Create(ctx context.Context, alert Alert) error
	// Resolve stamps resolvedAt on the alert identified by id.
	Resolve(ctx context.Context, id string, resolvedAt time.Time) error
	// FindOpenByRule returns the currently-open (ResolvedAt == nil) alert for ruleID, if any.
	FindOpenByRule(ctx context.Context, ruleID string) (Alert, bool, error)
	// FindOpenSystemAlert returns the currently-open, rule-less (RuleID == nil) alert for
	// sourceID, if any — the connectivity/status alert tracked by EvaluateSourceStatus.
	FindOpenSystemAlert(ctx context.Context, sourceID string) (Alert, bool, error)
	// List returns alerts filtered by status: "" (all), "active" (ResolvedAt == nil), or
	// "resolved" (ResolvedAt != nil).
	List(ctx context.Context, status string) ([]Alert, error)
}

// AlertRuleRepository is the persistence port for AlertRule.
type AlertRuleRepository interface {
	Create(ctx context.Context, rule AlertRule) error
	Update(ctx context.Context, rule AlertRule) error
	Get(ctx context.Context, id string) (AlertRule, error)
	// List returns every rule, optionally filtered to one sourceID ("" = all).
	List(ctx context.Context, sourceID string) ([]AlertRule, error)
	// ListEnabledByChannel returns every enabled rule referencing channelID — used to block
	// deleting a channel still in use.
	ListEnabledByChannel(ctx context.Context, channelID string) ([]AlertRule, error)
	Delete(ctx context.Context, id string) error
}

// NotificationChannelRepository is the persistence port for NotificationChannel.
type NotificationChannelRepository interface {
	Create(ctx context.Context, channel NotificationChannel) error
	Update(ctx context.Context, channel NotificationChannel) error
	Get(ctx context.Context, id string) (NotificationChannel, error)
	List(ctx context.Context) ([]NotificationChannel, error)
	Delete(ctx context.Context, id string) error
}
