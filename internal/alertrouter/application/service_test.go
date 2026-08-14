package application_test

import (
	"context"
	"testing"
	"time"

	"sre-kit/internal/alertrouter/application"
	"sre-kit/internal/alertrouter/domain"
)

// --- fakes ---

type fakeAlerts struct {
	byID map[string]domain.Alert
}

func newFakeAlerts() *fakeAlerts { return &fakeAlerts{byID: make(map[string]domain.Alert)} }

func (f *fakeAlerts) Create(_ context.Context, alert domain.Alert) error {
	f.byID[alert.ID] = alert
	return nil
}
func (f *fakeAlerts) Resolve(_ context.Context, id string, resolvedAt time.Time) error {
	a := f.byID[id]
	a.ResolvedAt = &resolvedAt
	f.byID[id] = a
	return nil
}
func (f *fakeAlerts) FindOpenByRule(_ context.Context, ruleID string) (domain.Alert, bool, error) {
	for _, a := range f.byID {
		if a.RuleID != nil && *a.RuleID == ruleID && a.ResolvedAt == nil {
			return a, true, nil
		}
	}
	return domain.Alert{}, false, nil
}
func (f *fakeAlerts) FindOpenSystemAlert(_ context.Context, sourceID string) (domain.Alert, bool, error) {
	for _, a := range f.byID {
		if a.RuleID == nil && a.SourceID == sourceID && a.ResolvedAt == nil {
			return a, true, nil
		}
	}
	return domain.Alert{}, false, nil
}
func (f *fakeAlerts) List(_ context.Context, status string) ([]domain.Alert, error) {
	var out []domain.Alert
	for _, a := range f.byID {
		switch status {
		case "active":
			if a.ResolvedAt != nil {
				continue
			}
		case "resolved":
			if a.ResolvedAt == nil {
				continue
			}
		}
		out = append(out, a)
	}
	return out, nil
}

type fakeRules struct {
	byID map[string]domain.AlertRule
}

func newFakeRules() *fakeRules { return &fakeRules{byID: make(map[string]domain.AlertRule)} }

func (f *fakeRules) Create(_ context.Context, rule domain.AlertRule) error {
	f.byID[rule.ID] = rule
	return nil
}
func (f *fakeRules) Update(_ context.Context, rule domain.AlertRule) error {
	if _, ok := f.byID[rule.ID]; !ok {
		return domain.ErrRuleNotFound
	}
	f.byID[rule.ID] = rule
	return nil
}
func (f *fakeRules) Get(_ context.Context, id string) (domain.AlertRule, error) {
	rule, ok := f.byID[id]
	if !ok {
		return domain.AlertRule{}, domain.ErrRuleNotFound
	}
	return rule, nil
}
func (f *fakeRules) List(_ context.Context, sourceID string) ([]domain.AlertRule, error) {
	var out []domain.AlertRule
	for _, r := range f.byID {
		if sourceID == "" || r.SourceID == sourceID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeRules) ListEnabledByChannel(_ context.Context, channelID string) ([]domain.AlertRule, error) {
	var out []domain.AlertRule
	for _, r := range f.byID {
		if r.NotifyChannelID == channelID && r.Enabled {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeRules) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return domain.ErrRuleNotFound
	}
	delete(f.byID, id)
	return nil
}

type fakeChannels struct {
	byID map[string]domain.NotificationChannel
}

func newFakeChannels() *fakeChannels {
	return &fakeChannels{byID: make(map[string]domain.NotificationChannel)}
}

func (f *fakeChannels) Create(_ context.Context, c domain.NotificationChannel) error {
	f.byID[c.ID] = c
	return nil
}
func (f *fakeChannels) Update(_ context.Context, c domain.NotificationChannel) error {
	if _, ok := f.byID[c.ID]; !ok {
		return domain.ErrChannelNotFound
	}
	f.byID[c.ID] = c
	return nil
}
func (f *fakeChannels) Get(_ context.Context, id string) (domain.NotificationChannel, error) {
	c, ok := f.byID[id]
	if !ok {
		return domain.NotificationChannel{}, domain.ErrChannelNotFound
	}
	return c, nil
}
func (f *fakeChannels) List(_ context.Context) ([]domain.NotificationChannel, error) {
	var out []domain.NotificationChannel
	for _, c := range f.byID {
		out = append(out, c)
	}
	return out, nil
}
func (f *fakeChannels) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return domain.ErrChannelNotFound
	}
	delete(f.byID, id)
	return nil
}

type fakeSecrets struct {
	byRef map[string]string
	seq   int
}

func newFakeSecrets() *fakeSecrets { return &fakeSecrets{byRef: make(map[string]string)} }

func (f *fakeSecrets) Put(value string) (string, error) {
	f.seq++
	ref := "ref-" + string(rune('a'+f.seq))
	f.byRef[ref] = value
	return ref, nil
}
func (f *fakeSecrets) Get(ref string) (string, error) {
	v, ok := f.byRef[ref]
	if !ok {
		return "", domain.ErrChannelNotFound
	}
	return v, nil
}
func (f *fakeSecrets) Delete(ref string) error {
	delete(f.byRef, ref)
	return nil
}

type fakeNotifier struct {
	sent []string
}

func (f *fakeNotifier) Send(_ context.Context, botToken, chatID, message string) error {
	f.sent = append(f.sent, botToken+"|"+chatID+"|"+message)
	return nil
}

type fakePublisher struct {
	frames []application.Frame
}

func (f *fakePublisher) Publish(frame application.Frame) {
	f.frames = append(f.frames, frame)
}

// --- tests ---

func TestService_EvaluateMetric_FiresAndResolves(t *testing.T) {
	ctx := context.Background()
	alerts, rules, channels, secrets := newFakeAlerts(), newFakeRules(), newFakeChannels(), newFakeSecrets()
	notifier, publisher := &fakeNotifier{}, &fakePublisher{}

	channel, err := application.NewService(alerts, rules, channels, secrets, application.WithNotifier(notifier), application.WithPublisher(publisher)).
		CreateChannel(ctx, "telegram", "chat-1", "bot-token-1")
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	svc := application.NewService(alerts, rules, channels, secrets, application.WithNotifier(notifier), application.WithPublisher(publisher))
	rule, err := svc.CreateRule(ctx, "src-1", "cpu.usage_percent", ">", "90", 0, channel.ID)
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	if err := svc.EvaluateMetric(ctx, "src-1", "cpu.usage_percent", 95); err != nil {
		t.Fatalf("EvaluateMetric (fire): %v", err)
	}
	active, _ := svc.ListAlerts(ctx, "active")
	if len(active) != 1 {
		t.Fatalf("active alerts = %d, want 1", len(active))
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("notifications sent = %d, want 1", len(notifier.sent))
	}
	if len(publisher.frames) != 1 || publisher.frames[0].Type != "alert" {
		t.Fatalf("frames = %+v", publisher.frames)
	}

	// Re-evaluating while still matched must not open a second alert.
	if err := svc.EvaluateMetric(ctx, "src-1", "cpu.usage_percent", 96); err != nil {
		t.Fatalf("EvaluateMetric (still firing): %v", err)
	}
	active, _ = svc.ListAlerts(ctx, "active")
	if len(active) != 1 {
		t.Fatalf("active alerts after re-eval = %d, want 1", len(active))
	}

	// Value drops below threshold -> resolves.
	if err := svc.EvaluateMetric(ctx, "src-1", "cpu.usage_percent", 10); err != nil {
		t.Fatalf("EvaluateMetric (resolve): %v", err)
	}
	active, _ = svc.ListAlerts(ctx, "active")
	if len(active) != 0 {
		t.Fatalf("active alerts after resolve = %d, want 0", len(active))
	}
	resolved, _ := svc.ListAlerts(ctx, "resolved")
	if len(resolved) != 1 {
		t.Fatalf("resolved alerts = %d, want 1", len(resolved))
	}
	if len(notifier.sent) != 2 {
		t.Fatalf("notifications sent after resolve = %d, want 2", len(notifier.sent))
	}

	_ = rule
}

func TestService_EvaluateMetric_Debounce(t *testing.T) {
	ctx := context.Background()
	alerts, rules, channels, secrets := newFakeAlerts(), newFakeRules(), newFakeChannels(), newFakeSecrets()
	svc := application.NewService(alerts, rules, channels, secrets)

	if _, err := svc.CreateRule(ctx, "src-1", "cpu.usage_percent", ">", "90", 60, ""); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	if err := svc.EvaluateMetric(ctx, "src-1", "cpu.usage_percent", 95); err != nil {
		t.Fatalf("EvaluateMetric: %v", err)
	}
	active, _ := svc.ListAlerts(ctx, "active")
	if len(active) != 0 {
		t.Fatalf("active alerts before debounce elapses = %d, want 0", len(active))
	}
}

func TestService_EvaluateCheck_StatusIs(t *testing.T) {
	ctx := context.Background()
	alerts, rules, channels, secrets := newFakeAlerts(), newFakeRules(), newFakeChannels(), newFakeSecrets()
	svc := application.NewService(alerts, rules, channels, secrets)

	if _, err := svc.CreateRule(ctx, "src-1", "uptime.http", "status_is", "critical", 0, ""); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if err := svc.EvaluateCheck(ctx, "src-1", "uptime.http", "critical"); err != nil {
		t.Fatalf("EvaluateCheck (fire): %v", err)
	}
	active, _ := svc.ListAlerts(ctx, "active")
	if len(active) != 1 {
		t.Fatalf("active alerts = %d, want 1", len(active))
	}
	if err := svc.EvaluateCheck(ctx, "src-1", "uptime.http", "ok"); err != nil {
		t.Fatalf("EvaluateCheck (resolve): %v", err)
	}
	active, _ = svc.ListAlerts(ctx, "active")
	if len(active) != 0 {
		t.Fatalf("active alerts after resolve = %d, want 0", len(active))
	}
}

func TestService_EvaluateSourceStatus(t *testing.T) {
	ctx := context.Background()
	alerts, rules, channels, secrets := newFakeAlerts(), newFakeRules(), newFakeChannels(), newFakeSecrets()
	svc := application.NewService(alerts, rules, channels, secrets)

	// "error" fires immediately, no debounce.
	if err := svc.EvaluateSourceStatus(ctx, "src-1", "error"); err != nil {
		t.Fatalf("EvaluateSourceStatus(error): %v", err)
	}
	active, _ := svc.ListAlerts(ctx, "active")
	if len(active) != 1 {
		t.Fatalf("active after error = %d, want 1", len(active))
	}

	// "ok" resolves it.
	if err := svc.EvaluateSourceStatus(ctx, "src-1", "ok"); err != nil {
		t.Fatalf("EvaluateSourceStatus(ok): %v", err)
	}
	active, _ = svc.ListAlerts(ctx, "active")
	if len(active) != 0 {
		t.Fatalf("active after ok = %d, want 0", len(active))
	}

	// "unreachable" debounces: first two calls must not fire (< 3 consecutive, < 5 min).
	for i := 0; i < 2; i++ {
		if err := svc.EvaluateSourceStatus(ctx, "src-2", "unreachable"); err != nil {
			t.Fatalf("EvaluateSourceStatus(unreachable) #%d: %v", i, err)
		}
	}
	active, _ = svc.ListAlerts(ctx, "active")
	if len(active) != 0 {
		t.Fatalf("active after 2 unreachable = %d, want 0", len(active))
	}
	// Third consecutive failure fires.
	if err := svc.EvaluateSourceStatus(ctx, "src-2", "unreachable"); err != nil {
		t.Fatalf("EvaluateSourceStatus(unreachable) #3: %v", err)
	}
	active, _ = svc.ListAlerts(ctx, "active")
	if len(active) != 1 {
		t.Fatalf("active after 3rd unreachable = %d, want 1", len(active))
	}
}

func TestService_DeleteChannel_BlockedWhenInUse(t *testing.T) {
	ctx := context.Background()
	alerts, rules, channels, secrets := newFakeAlerts(), newFakeRules(), newFakeChannels(), newFakeSecrets()
	svc := application.NewService(alerts, rules, channels, secrets)

	channel, err := svc.CreateChannel(ctx, "telegram", "chat-1", "bot-token-1")
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := svc.CreateRule(ctx, "src-1", "cpu.usage_percent", ">", "90", 0, channel.ID); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if err := svc.DeleteChannel(ctx, channel.ID); err != domain.ErrChannelInUse {
		t.Fatalf("DeleteChannel while in use: err = %v, want ErrChannelInUse", err)
	}
}
