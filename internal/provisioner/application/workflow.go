package application

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"

	"sre-kit/internal/provisioner/domain"
)

// httpDoer is the narrow http.Client surface Service.runHTTPCall needs — lets tests fake HTTP
// bootstrap calls without a real listener beyond httptest.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// templateData is the Go text/template root for every rendered string in a run (compose file,
// bootstrap steps, produced source config): host connection info, the generated admin credential,
// and any value captured by an earlier http_call bootstrap step (docs/SPEC.md §12.3).
type templateData map[string]string

// Service orchestrates the provisioning workflow: render a preset's compose file, deploy it over
// SSH, bootstrap the tool's admin account, and register the result as a Source
// (docs/SPEC.md §12.2). Every dependency is a narrow port so this stays testable without a real
// SSH dial or HTTP listener.
type Service struct {
	runs          domain.Repository
	hosts         HostsLookup
	ssh           SSHRunner
	secrets       SecretsStore
	sourceCreator SourceCreator
	presetsDir    string
	now           func() time.Time
	genPassword   func() (string, error)
	httpClient    httpDoer
}

// Option configures optional Service dependencies not required by every caller/test.
type Option func(*Service)

// WithHTTPClient overrides the http.Client used for http_call bootstrap steps — tests wire a fake
// via httptest instead of the real network.
func WithHTTPClient(client httpDoer) Option {
	return func(s *Service) { s.httpClient = client }
}

// NewService wires a Service to its ports and any Options.
func NewService(runs domain.Repository, hosts HostsLookup, ssh SSHRunner, secrets SecretsStore, sourceCreator SourceCreator, presetsDir string, opts ...Option) *Service {
	s := &Service{
		runs:          runs,
		hosts:         hosts,
		ssh:           ssh,
		secrets:       secrets,
		sourceCreator: sourceCreator,
		presetsDir:    presetsDir,
		now:           time.Now,
		genPassword:   randomPassword,
		httpClient:    http.DefaultClient,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start creates a new Run for hostID+presetName and runs it to completion or first failure. v1
// runs synchronously within the HTTP request that triggered it (docs/SPEC.md §12.2) — there is no
// background job queue yet; a slow deploy blocks the request. Acceptable for a one-click, rarely
// repeated action; revisit if deploy latency becomes a real UX problem.
func (s *Service) Start(ctx context.Context, hostID, presetName string) (domain.Run, error) {
	if _, err := LoadPreset(s.presetsDir, presetName); err != nil {
		return domain.Run{}, err
	}
	run := domain.Run{
		ID:         uuid.NewString(),
		HostID:     hostID,
		PresetName: presetName,
		Status:     domain.StatusPending,
		StartedAt:  s.now(),
	}
	if err := s.runs.Create(ctx, run); err != nil {
		return domain.Run{}, fmt.Errorf("provisioner: create run: %w", err)
	}
	return s.advance(ctx, run), nil
}

// Retry resumes a failed run from its last completed Step (docs/SPEC.md §12.4's partial-failure
// requirement) rather than restarting from scratch.
func (s *Service) Retry(ctx context.Context, runID string) (domain.Run, error) {
	run, err := s.runs.Get(ctx, runID)
	if err != nil {
		return domain.Run{}, err
	}
	if run.Status != domain.StatusFailed {
		return domain.Run{}, domain.ErrNotRetryable
	}
	run.ErrorMessage = ""
	return s.advance(ctx, run), nil
}

// Get returns the run identified by id.
func (s *Service) Get(ctx context.Context, id string) (domain.Run, error) {
	return s.runs.Get(ctx, id)
}

// ListByHost returns every run for hostID, for polling status (docs/SPEC.md §4).
func (s *Service) ListByHost(ctx context.Context, hostID string) ([]domain.Run, error) {
	runs, err := s.runs.ListByHost(ctx, hostID)
	if err != nil {
		return nil, fmt.Errorf("provisioner: list runs: %w", err)
	}
	return runs, nil
}

// advance drives run through deploy -> bootstrap -> register, starting at whatever Step is already
// recorded (empty on a fresh Start, or the last completed step on a Retry). Each phase persists its
// Step before starting the next one, so a mid-phase failure leaves an accurate resume point.
func (s *Service) advance(ctx context.Context, run domain.Run) domain.Run {
	preset, err := LoadPreset(s.presetsDir, run.PresetName)
	if err != nil {
		return s.fail(ctx, run, err)
	}
	conn, err := s.hosts(ctx, run.HostID)
	if err != nil {
		return s.fail(ctx, run, err)
	}

	data := templateData{"HostAddress": conn.Address, "AdminEmail": "admin@sre-kit.local"}

	if run.Step == "" {
		run.Status = domain.StatusDeploying
		s.save(ctx, &run)

		composeContent, err := renderTemplate(preset.ComposeTemplate, data)
		if err != nil {
			return s.fail(ctx, run, fmt.Errorf("render compose template: %w", err))
		}
		deployDir := fmt.Sprintf(".sre-kit/%s", run.ID)
		if err := s.ssh.UploadFile(ctx, conn, deployDir+"/docker-compose.yml", []byte(composeContent)); err != nil {
			return s.fail(ctx, run, fmt.Errorf("upload compose file: %w", err))
		}
		if _, err := s.ssh.RunCommand(ctx, conn, fmt.Sprintf("cd %s && docker compose up -d", deployDir)); err != nil {
			return s.fail(ctx, run, fmt.Errorf("docker compose up: %w", err))
		}
		run.Step = domain.StepDeploy
		s.save(ctx, &run)
	}

	if run.Step == domain.StepDeploy {
		run.Status = domain.StatusBootstrapping
		s.save(ctx, &run)

		adminPassword, err := s.genPassword()
		if err != nil {
			return s.fail(ctx, run, err)
		}
		data["AdminPassword"] = adminPassword
		if err := s.runBootstrap(ctx, conn, preset, data); err != nil {
			return s.fail(ctx, run, fmt.Errorf("bootstrap: %w", err))
		}
		ref, err := s.secrets.Put(adminPassword)
		if err != nil {
			return s.fail(ctx, run, fmt.Errorf("store generated admin password: %w", err))
		}
		run.AdminPasswordSecretRef = ref
		run.Step = domain.StepBootstrap
		s.save(ctx, &run)
	}

	if run.Step == domain.StepBootstrap {
		run.Status = domain.StatusRegistering
		s.save(ctx, &run)

		// Resolve the credential actually set on the tool during bootstrap — on a fresh run this
		// is the password just generated above; on a Retry resumed straight into this phase, it's
		// whatever a prior, successful bootstrap attempt set (docs/SPEC.md §12: "a retried
		// registration reuses the credential actually set on the tool rather than generating a new
		// one").
		adminPassword, err := s.secrets.Get(run.AdminPasswordSecretRef)
		if err != nil {
			return s.fail(ctx, run, fmt.Errorf("resolve stored admin password: %w", err))
		}
		data["AdminPassword"] = adminPassword

		configFields := make(map[string]string, len(preset.Manifest.ProducesSourceConfigTemplate))
		for field, tmpl := range preset.Manifest.ProducesSourceConfigTemplate {
			rendered, err := renderTemplate(tmpl, data)
			if err != nil {
				return s.fail(ctx, run, fmt.Errorf("render source config field %q: %w", field, err))
			}
			configFields[field] = rendered
		}
		configJSON, err := json.Marshal(configFields)
		if err != nil {
			return s.fail(ctx, run, fmt.Errorf("encode source config: %w", err))
		}
		sourceID, err := s.sourceCreator(ctx, preset.Manifest.ProducesAdapter, string(configJSON))
		if err != nil {
			return s.fail(ctx, run, fmt.Errorf("register source: %w", err))
		}

		run.ProducedSourceID = sourceID
		run.Step = domain.StepRegister
		run.Status = domain.StatusDone
		run.ErrorMessage = ""
		finishedAt := s.now()
		run.FinishedAt = &finishedAt
		s.save(ctx, &run)
	}

	return run
}

func (s *Service) fail(ctx context.Context, run domain.Run, err error) domain.Run {
	run.Status = domain.StatusFailed
	run.ErrorMessage = err.Error()
	s.save(ctx, &run)
	return run
}

// save is best-effort: if persisting fails mid-workflow, the in-memory run is still returned to the
// caller for this call, and a subsequent Retry will simply re-read whatever was last durably
// recorded (worst case, that means resuming from an earlier Step than this call actually reached —
// safe, since docker compose up and the bootstrap idempotency check tolerate re-running).
func (s *Service) save(ctx context.Context, run *domain.Run) {
	_ = s.runs.Update(ctx, *run)
}

// runBootstrap executes preset's bootstrap steps in order against conn/data, per docs/SPEC.md
// §12.3's two step types.
func (s *Service) runBootstrap(ctx context.Context, conn HostConn, preset domain.Preset, data templateData) error {
	baseURL, err := renderTemplate(preset.Manifest.BaseURLTemplate, data)
	if err != nil {
		return fmt.Errorf("render base_url_template: %w", err)
	}
	for i, step := range preset.Bootstrap {
		switch step.Type {
		case domain.BootstrapSSHCommand:
			command, err := renderTemplate(step.Command, data)
			if err != nil {
				return fmt.Errorf("step %d: render command: %w", i, err)
			}
			if _, err := s.ssh.RunCommand(ctx, conn, command); err != nil {
				return fmt.Errorf("step %d: %w", i, err)
			}
		case domain.BootstrapHTTPCall:
			if err := s.runHTTPCall(ctx, baseURL, step, data); err != nil {
				return fmt.Errorf("step %d: %w", i, err)
			}
		default:
			return fmt.Errorf("step %d: unknown bootstrap step type %q", i, step.Type)
		}
	}
	return nil
}

// runHTTPCall renders and issues one http_call bootstrap step against baseURL, optionally
// capturing a field from the JSON response into data for later steps (docs/SPEC.md §12.3).
func (s *Service) runHTTPCall(ctx context.Context, baseURL string, step domain.BootstrapStep, data templateData) error {
	path, err := renderTemplate(step.PathTemplate, data)
	if err != nil {
		return fmt.Errorf("render path_template: %w", err)
	}
	var body io.Reader
	if step.BodyTemplate != "" {
		rendered, err := renderTemplate(step.BodyTemplate, data)
		if err != nil {
			return fmt.Errorf("render body_template: %w", err)
		}
		body = strings.NewReader(rendered)
	}
	req, err := http.NewRequestWithContext(ctx, step.Method, baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if step.AuthFromField != "" {
		token, ok := data[step.AuthFromField]
		if !ok {
			return fmt.Errorf("auth_from_field %q was not captured by an earlier step", step.AuthFromField)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
	}

	if step.CaptureField != "" {
		var parsed map[string]any
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return fmt.Errorf("capture %q: parse response: %w", step.CaptureField, err)
		}
		value, ok := parsed[step.CaptureField]
		if !ok {
			return fmt.Errorf("capture %q: field not present in response", step.CaptureField)
		}
		as := step.CaptureAs
		if as == "" {
			as = step.CaptureField
		}
		data[as] = fmt.Sprint(value)
	}
	return nil
}

func renderTemplate(tmplStr string, data templateData) (string, error) {
	tmpl, err := template.New("t").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// randomPassword generates a 48-hex-char (24 random bytes) admin password for a freshly
// provisioned tool. Never derived from any user input — always fully random.
func randomPassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random password: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
