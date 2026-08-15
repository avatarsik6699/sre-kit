// Command server is the composition root: manual wiring of config, storage, and every bounded
// context's HTTP handlers onto the shared httpserver mount point. No DI framework — see
// docs/STACK.md § Backend Architecture.
//
// @title                       sre-kit API
// @version                     1.0
// @description                 Self-hosted SRE/observability aggregator core API.
// @BasePath                    /
// @securitydefinitions.apikey  SessionCookie
// @in                          cookie
// @name                        session
//
//go:generate swag init -g cmd/server/main.go --dir ../../ --output ../../.swagger-gen --parseInternal --parseDependency
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	adapterengineapp "sre-kit/internal/adapterengine/application"
	adapterenginedomain "sre-kit/internal/adapterengine/domain"
	adapterengineinfra "sre-kit/internal/adapterengine/infrastructure"
	adapterenginehttp "sre-kit/internal/adapterengine/interfaces/http"
	alertrouterapp "sre-kit/internal/alertrouter/application"
	alertrouterinfra "sre-kit/internal/alertrouter/infrastructure"
	alertrouterhttp "sre-kit/internal/alertrouter/interfaces/http"
	authapp "sre-kit/internal/auth/application"
	authhttp "sre-kit/internal/auth/interfaces/http"
	hostsapp "sre-kit/internal/hosts/application"
	hostsinfra "sre-kit/internal/hosts/infrastructure"
	hostshttp "sre-kit/internal/hosts/interfaces/http"
	"sre-kit/internal/notify/telegram"
	"sre-kit/internal/platform/config"
	"sre-kit/internal/platform/db"
	"sre-kit/internal/platform/httpserver"
	"sre-kit/internal/platform/secrets"
	"sre-kit/internal/platform/wshub"
	provisionerapp "sre-kit/internal/provisioner/application"
	provisionerinfra "sre-kit/internal/provisioner/infrastructure"
	provisionerhttp "sre-kit/internal/provisioner/interfaces/http"
	sourcesapp "sre-kit/internal/sources/application"
	sourcesdomain "sre-kit/internal/sources/domain"
	sourcesinfra "sre-kit/internal/sources/infrastructure"
	sourceshttp "sre-kit/internal/sources/interfaces/http"
	telemetryapp "sre-kit/internal/telemetry/application"
	telemetryinfra "sre-kit/internal/telemetry/infrastructure"
	telemetryhttp "sre-kit/internal/telemetry/interfaces/http"
)

// hubPublisher adapts telemetryapp.Publisher onto wshub.Hub — the composition-root translation
// between telemetry/application's dependency-free Frame and wshub's concrete Frame, per
// docs/STACK.md's ports-over-direct-imports rule.
type hubPublisher struct {
	hub *wshub.Hub
}

func (p hubPublisher) Publish(frame telemetryapp.Frame) {
	p.hub.Publish(wshub.Frame{Type: frame.Type, SourceID: frame.SourceID, Payload: frame.Payload})
}

// alertHubPublisher is hubPublisher's alertrouter counterpart — same composition-root translation
// pattern, kept as a separate type since alertrouterapp.Frame and telemetryapp.Frame are distinct
// dependency-free types (ports-over-direct-imports, docs/STACK.md).
type alertHubPublisher struct {
	hub *wshub.Hub
}

func (p alertHubPublisher) Publish(frame alertrouterapp.Frame) {
	p.hub.Publish(wshub.Frame{Type: frame.Type, SourceID: frame.SourceID, Payload: frame.Payload})
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer sqlDB.Close()

	if err := db.Migrate(sqlDB); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	secretsStore, err := secrets.Open(cfg.SecretsPath, cfg.SecretsKey)
	if err != nil {
		log.Fatalf("secrets: %v", err)
	}

	srv := httpserver.New(cfg.Addr)
	srv.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authService := authapp.NewService(secretsStore)
	if generatedPassword, err := authService.EnsureAdminPassword(context.Background()); err != nil {
		log.Fatalf("auth: bootstrap admin password: %v", err)
	} else if generatedPassword != "" {
		log.Printf("generated admin password (save this — it is not shown again): %s", generatedPassword)
	}
	authhttp.NewHandlers(authService).Register(srv.Mux)
	srv.Use(authhttp.RequireSession(authService))

	// adapterConfigSchema resolves an adapter's config_schema by name so sourcesService can tell
	// which config fields are secrets (docs/changes/07-source-secret-ref-fix.md) — reuses the same
	// ListInstalled lookup reconcileSchedule does below, by adapter name rather than pull-mode
	// job info. sourcesService deliberately doesn't import adapterengine directly (ports, not
	// direct imports).
	adapterConfigSchema := func(_ context.Context, adapterName string) (json.RawMessage, error) {
		installed, err := adapterengineapp.ListInstalled(cfg.AdaptersDir)
		if err != nil {
			return nil, err
		}
		for _, adapter := range installed {
			if adapter.Manifest.Name == adapterName {
				return adapter.Manifest.ConfigSchema, nil
			}
		}
		return nil, fmt.Errorf("adapter %q not installed", adapterName)
	}

	sourcesRepo := sourcesinfra.NewSQLiteRepository(sqlDB)
	sourcesService := sourcesapp.NewService(sourcesRepo,
		sourcesapp.WithSecrets(secretsStore),
		sourcesapp.WithAdapterConfigSchemas(adapterConfigSchema),
	)

	hub := wshub.New()

	alertrouterRepos := alertrouterinfra.NewSQLiteRepository(sqlDB)
	alertrouterService := alertrouterapp.NewService(
		alertrouterRepos.Alerts, alertrouterRepos.Rules, alertrouterRepos.Channels, secretsStore,
		alertrouterapp.WithNotifier(telegram.NewClient()),
		alertrouterapp.WithPublisher(alertHubPublisher{hub: hub}),
	)
	alertrouterhttp.NewHandlers(alertrouterService).Register(srv.Mux)

	telemetryRepos := telemetryinfra.NewSQLiteRepository(sqlDB)
	telemetryService := telemetryapp.NewService(
		telemetryRepos.Metrics, telemetryRepos.Checks, telemetryRepos.Events,
		telemetryapp.WithPublisher(hubPublisher{hub: hub}),
		telemetryapp.WithSourceStatusUpdater(sourcesService),
		telemetryapp.WithAlertEvaluator(alertrouterService),
	)
	telemetryhttp.NewHandlers(telemetryService, hub).Register(srv.Mux)

	// Adapter engine: pull-mode sources are kept scheduled in sync with source state via
	// sourcesService.OnChange below (see docs/changes/01-core-skeleton.md Architect Review Notes
	// R1). Stream-mode sources aren't wired yet — no stream adapter exists to exercise it.
	adapterRunner := adapterengineapp.NewRunner(
		adapterengineinfra.NewProcessSpawner(),
		telemetryService,
		func(ctx context.Context, sourceID string) error {
			_, err := sourcesService.Disable(ctx, sourceID)
			return err
		},
	)
	scheduler := adapterengineapp.NewScheduler(adapterRunner)

	// schedulerCtx is the scheduler's own lifetime — deliberately not the caller's request
	// context. A scheduled job must keep running after the HTTP request that created/enabled its
	// source has completed and its context has been cancelled.
	schedulerCtx := context.Background()

	reconcileSchedule := func(_ context.Context, source sourcesdomain.Source, deleted bool) {
		if deleted || !source.Enabled {
			scheduler.Cancel(source.ID)
			return
		}
		installed, err := adapterengineapp.ListInstalled(cfg.AdaptersDir)
		if err != nil {
			log.Printf("adapterengine: list installed adapters: %v", err)
			return
		}
		for _, adapter := range installed {
			if adapter.Manifest.Name != source.AdapterName {
				continue
			}
			if adapter.Manifest.Mode != adapterenginedomain.ModePull {
				log.Printf("adapterengine: source %s uses non-pull adapter %q — stream-mode scheduling isn't wired yet", source.ID, adapter.Manifest.Name)
				return
			}
			// Convention (matches Dockerfile): an adapter's executable is named after its
			// directory, e.g. adapters/stub/stub.
			command := filepath.Join(adapter.Dir, filepath.Base(adapter.Dir))
			// sources.config_json only ever stores a secret_ref (docs/SPEC.md §3) — resolve it to
			// the real plaintext (SSH password/key, etc.) here, at spawn time, so the subprocess's
			// stdin carries a usable credential. The ref never leaves this process otherwise.
			resolvedConfig, err := secrets.ResolveConfig(secretsStore, adapter.Manifest.ConfigSchema, source.ConfigJSON)
			if err != nil {
				log.Printf("adapterengine: source %s: resolve config secrets: %v", source.ID, err)
				return
			}
			scheduler.Schedule(schedulerCtx, adapterengineapp.PullJob{
				SourceID: source.ID,
				Command:  command,
				Config:   resolvedConfig,
			})
			return
		}
		log.Printf("adapterengine: source %s references unknown/uninstalled adapter %q", source.ID, source.AdapterName)
	}
	sourcesService.OnChange(reconcileSchedule)

	bootstrapSources, err := sourcesService.List(schedulerCtx)
	if err != nil {
		log.Fatalf("sources: list: %v", err)
	}
	for _, source := range bootstrapSources {
		reconcileSchedule(schedulerCtx, source, false)
	}

	sourceshttp.NewHandlers(sourcesService).Register(srv.Mux)
	adapterenginehttp.NewHandlers(cfg.AdaptersDir).Register(srv.Mux)

	// Observability Auto-Provisioning (docs/SPEC.md §12, added post-M6). internal/hosts and
	// internal/provisioner deliberately don't import internal/sources directly — sourceCreator
	// below is the same ports-not-direct-imports pattern used throughout this composition root.
	hostsRepo := hostsinfra.NewSQLiteRepository(sqlDB)
	hostsService := hostsapp.NewService(hostsRepo, secretsStore, hostsapp.WithProber(hostsinfra.NewSSHProber()))
	hostshttp.NewHandlers(hostsService).Register(srv.Mux)

	// hostsLookup resolves a Host's SSH connection info for the provisioner — the private key is
	// resolved from its secret_ref here, at the point of use, same rule §3 gives adapter config
	// (never persisted or logged past this point). ExpectedFingerprint carries whatever
	// internal/hosts's CheckConnection has pinned; empty until the host has been checked at least
	// once, which provisionerinfra.SSHRunner refuses to dial without (docs/SPEC.md §12.4).
	hostsLookup := func(ctx context.Context, hostID string) (provisionerapp.HostConn, error) {
		host, err := hostsService.Get(ctx, hostID)
		if err != nil {
			return provisionerapp.HostConn{}, err
		}
		keyPEM, err := secretsStore.Get(host.SSHKeySecretRef)
		if err != nil {
			return provisionerapp.HostConn{}, fmt.Errorf("hosts: resolve ssh key: %w", err)
		}
		return provisionerapp.HostConn{
			ID:                  host.ID,
			Address:             host.Address,
			Port:                host.SSHPort,
			User:                host.SSHUser,
			PrivateKeyPEM:       keyPEM,
			ExpectedFingerprint: host.HostKeyFingerprint,
		}, nil
	}
	sourceCreator := func(ctx context.Context, adapterName string, configJSON string) (string, error) {
		source, err := sourcesService.Create(ctx, adapterName, configJSON)
		if err != nil {
			return "", err
		}
		return source.ID, nil
	}

	provisionerRepo := provisionerinfra.NewSQLiteRepository(sqlDB)
	provisionerService := provisionerapp.NewService(provisionerRepo, hostsLookup, provisionerinfra.NewSSHRunner(), secretsStore, sourceCreator, cfg.PresetsDir)
	provisionerhttp.NewHandlers(provisionerService, cfg.PresetsDir).Register(srv.Mux)

	log.Printf("listening on %s", cfg.Addr)
	if err := srv.Start(); err != nil {
		log.Fatalf("http server: %v", err)
	}
}
