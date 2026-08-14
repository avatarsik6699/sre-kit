// Package infrastructure implements the alertrouter repository ports (Alert, AlertRule,
// NotificationChannel) against SQLite.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sre-kit/internal/alertrouter/domain"
)

// Repositories bundles the three SQLite-backed repositories NewSQLiteRepository constructs, ready
// to hand to alertrouter/application.NewService.
type Repositories struct {
	Alerts   domain.AlertRepository
	Rules    domain.AlertRuleRepository
	Channels domain.NotificationChannelRepository
}

// NewSQLiteRepository wires all three alertrouter repository ports to the shared *sql.DB.
func NewSQLiteRepository(db *sql.DB) Repositories {
	return Repositories{
		Alerts:   &alertRepository{db: db},
		Rules:    &alertRuleRepository{db: db},
		Channels: &notificationChannelRepository{db: db},
	}
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func nullableString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// --- Alert ---

type alertRepository struct{ db *sql.DB }

func (r *alertRepository) Create(ctx context.Context, alert domain.Alert) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO alerts (id, source_id, rule_id, severity, message, created_at, resolved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		alert.ID, alert.SourceID, nullableString(alert.RuleID), alert.Severity, alert.Message,
		alert.CreatedAt, nullableTime(alert.ResolvedAt))
	if err != nil {
		return fmt.Errorf("alertrouter: insert alert: %w", err)
	}
	return nil
}

func (r *alertRepository) Resolve(ctx context.Context, id string, resolvedAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE alerts SET resolved_at = ? WHERE id = ?`, resolvedAt, id)
	if err != nil {
		return fmt.Errorf("alertrouter: resolve alert: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return domain.ErrAlertNotFound
	}
	return nil
}

func (r *alertRepository) FindOpenByRule(ctx context.Context, ruleID string) (domain.Alert, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, source_id, rule_id, severity, message, created_at, resolved_at
		FROM alerts WHERE rule_id = ? AND resolved_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, ruleID)
	return scanOptionalAlert(row)
}

func (r *alertRepository) FindOpenSystemAlert(ctx context.Context, sourceID string) (domain.Alert, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, source_id, rule_id, severity, message, created_at, resolved_at
		FROM alerts WHERE source_id = ? AND rule_id IS NULL AND resolved_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, sourceID)
	return scanOptionalAlert(row)
}

func (r *alertRepository) List(ctx context.Context, status string) ([]domain.Alert, error) {
	sqlQuery := `SELECT id, source_id, rule_id, severity, message, created_at, resolved_at FROM alerts WHERE 1=1`
	switch status {
	case "active":
		sqlQuery += ` AND resolved_at IS NULL`
	case "resolved":
		sqlQuery += ` AND resolved_at IS NOT NULL`
	}
	sqlQuery += ` ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("alertrouter: list alerts: %w", err)
	}
	defer rows.Close()

	var alerts []domain.Alert
	for rows.Next() {
		alert, err := scanAlert(rows)
		if err != nil {
			return nil, fmt.Errorf("alertrouter: scan alert: %w", err)
		}
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}

func scanOptionalAlert(row *sql.Row) (domain.Alert, bool, error) {
	alert, err := scanAlert(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Alert{}, false, nil
	}
	if err != nil {
		return domain.Alert{}, false, fmt.Errorf("alertrouter: find open alert: %w", err)
	}
	return alert, true, nil
}

func scanAlert(scanner rowScanner) (domain.Alert, error) {
	var alert domain.Alert
	var ruleID sql.NullString
	var resolvedAt sql.NullTime
	if err := scanner.Scan(&alert.ID, &alert.SourceID, &ruleID, &alert.Severity, &alert.Message, &alert.CreatedAt, &resolvedAt); err != nil {
		return domain.Alert{}, err
	}
	if ruleID.Valid {
		alert.RuleID = &ruleID.String
	}
	if resolvedAt.Valid {
		alert.ResolvedAt = &resolvedAt.Time
	}
	return alert, nil
}

// --- AlertRule ---

type alertRuleRepository struct{ db *sql.DB }

func (r *alertRuleRepository) Create(ctx context.Context, rule domain.AlertRule) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO alert_rules (id, source_id, target_name, condition, threshold, debounce_seconds, notify_channel_id, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.SourceID, rule.TargetName, rule.Condition, rule.Threshold, rule.DebounceSeconds, rule.NotifyChannelID, rule.Enabled)
	if err != nil {
		return fmt.Errorf("alertrouter: insert alert rule: %w", err)
	}
	return nil
}

func (r *alertRuleRepository) Update(ctx context.Context, rule domain.AlertRule) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE alert_rules SET target_name = ?, condition = ?, threshold = ?, debounce_seconds = ?, notify_channel_id = ?, enabled = ?
		WHERE id = ?`,
		rule.TargetName, rule.Condition, rule.Threshold, rule.DebounceSeconds, rule.NotifyChannelID, rule.Enabled, rule.ID)
	if err != nil {
		return fmt.Errorf("alertrouter: update alert rule: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return domain.ErrRuleNotFound
	}
	return nil
}

func (r *alertRuleRepository) Get(ctx context.Context, id string) (domain.AlertRule, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, source_id, target_name, condition, threshold, debounce_seconds, notify_channel_id, enabled
		FROM alert_rules WHERE id = ?`, id)
	rule, err := scanRule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AlertRule{}, domain.ErrRuleNotFound
	}
	if err != nil {
		return domain.AlertRule{}, fmt.Errorf("alertrouter: get alert rule: %w", err)
	}
	return rule, nil
}

func (r *alertRuleRepository) List(ctx context.Context, sourceID string) ([]domain.AlertRule, error) {
	sqlQuery := `SELECT id, source_id, target_name, condition, threshold, debounce_seconds, notify_channel_id, enabled FROM alert_rules WHERE 1=1`
	var args []any
	if sourceID != "" {
		sqlQuery += ` AND source_id = ?`
		args = append(args, sourceID)
	}
	sqlQuery += ` ORDER BY id`

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("alertrouter: list alert rules: %w", err)
	}
	defer rows.Close()

	var rules []domain.AlertRule
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("alertrouter: scan alert rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *alertRuleRepository) ListEnabledByChannel(ctx context.Context, channelID string) ([]domain.AlertRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, source_id, target_name, condition, threshold, debounce_seconds, notify_channel_id, enabled
		FROM alert_rules WHERE notify_channel_id = ? AND enabled = 1`, channelID)
	if err != nil {
		return nil, fmt.Errorf("alertrouter: list rules by channel: %w", err)
	}
	defer rows.Close()

	var rules []domain.AlertRule
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("alertrouter: scan alert rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *alertRuleRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM alert_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("alertrouter: delete alert rule: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return domain.ErrRuleNotFound
	}
	return nil
}

func scanRule(scanner rowScanner) (domain.AlertRule, error) {
	var rule domain.AlertRule
	var notifyChannelID sql.NullString
	if err := scanner.Scan(&rule.ID, &rule.SourceID, &rule.TargetName, &rule.Condition, &rule.Threshold, &rule.DebounceSeconds, &notifyChannelID, &rule.Enabled); err != nil {
		return domain.AlertRule{}, err
	}
	rule.NotifyChannelID = notifyChannelID.String
	return rule, nil
}

// --- NotificationChannel ---

type notificationChannelRepository struct{ db *sql.DB }

func (r *notificationChannelRepository) Create(ctx context.Context, channel domain.NotificationChannel) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notification_channels (id, type, config_json, secret_ref, enabled)
		VALUES (?, ?, ?, ?, ?)`,
		channel.ID, channel.Type, channel.ConfigJSON, channel.SecretRef, channel.Enabled)
	if err != nil {
		return fmt.Errorf("alertrouter: insert notification channel: %w", err)
	}
	return nil
}

func (r *notificationChannelRepository) Update(ctx context.Context, channel domain.NotificationChannel) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE notification_channels SET config_json = ?, secret_ref = ?, enabled = ? WHERE id = ?`,
		channel.ConfigJSON, channel.SecretRef, channel.Enabled, channel.ID)
	if err != nil {
		return fmt.Errorf("alertrouter: update notification channel: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return domain.ErrChannelNotFound
	}
	return nil
}

func (r *notificationChannelRepository) Get(ctx context.Context, id string) (domain.NotificationChannel, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, type, config_json, secret_ref, enabled FROM notification_channels WHERE id = ?`, id)
	channel, err := scanChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NotificationChannel{}, domain.ErrChannelNotFound
	}
	if err != nil {
		return domain.NotificationChannel{}, fmt.Errorf("alertrouter: get notification channel: %w", err)
	}
	return channel, nil
}

func (r *notificationChannelRepository) List(ctx context.Context) ([]domain.NotificationChannel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, type, config_json, secret_ref, enabled FROM notification_channels ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("alertrouter: list notification channels: %w", err)
	}
	defer rows.Close()

	var channels []domain.NotificationChannel
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, fmt.Errorf("alertrouter: scan notification channel: %w", err)
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func (r *notificationChannelRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM notification_channels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("alertrouter: delete notification channel: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return domain.ErrChannelNotFound
	}
	return nil
}

func scanChannel(scanner rowScanner) (domain.NotificationChannel, error) {
	var channel domain.NotificationChannel
	if err := scanner.Scan(&channel.ID, &channel.Type, &channel.ConfigJSON, &channel.SecretRef, &channel.Enabled); err != nil {
		return domain.NotificationChannel{}, err
	}
	return channel, nil
}
