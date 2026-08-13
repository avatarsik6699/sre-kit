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
	"log"
	"net/http"
	"path/filepath"

	adapterengineapp "sre-kit/internal/adapterengine/application"
	adapterenginedomain "sre-kit/internal/adapterengine/domain"
	adapterengineinfra "sre-kit/internal/adapterengine/infrastructure"
	adapterenginehttp "sre-kit/internal/adapterengine/interfaces/http"
	authapp "sre-kit/internal/auth/application"
	authhttp "sre-kit/internal/auth/interfaces/http"
	"sre-kit/internal/platform/config"
	"sre-kit/internal/platform/db"
	"sre-kit/internal/platform/httpserver"
	"sre-kit/internal/platform/secrets"
	sourcesapp "sre-kit/internal/sources/application"
	sourcesdomain "sre-kit/internal/sources/domain"
	sourcesinfra "sre-kit/internal/sources/infrastructure"
	sourceshttp "sre-kit/internal/sources/interfaces/http"
	telemetryapp "sre-kit/internal/telemetry/application"
	telemetryinfra "sre-kit/internal/telemetry/infrastructure"
	telemetryhttp "sre-kit/internal/telemetry/interfaces/http"
)

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

	telemetryRepos := telemetryinfra.NewSQLiteRepository(sqlDB)
	telemetryService := telemetryapp.NewService(telemetryRepos.Metrics, telemetryRepos.Checks, telemetryRepos.Events)
	telemetryhttp.NewHandlers(telemetryService).Register(srv.Mux)

	sourcesRepo := sourcesinfra.NewSQLiteRepository(sqlDB)
	sourcesService := sourcesapp.NewService(sourcesRepo)

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
			scheduler.Schedule(schedulerCtx, adapterengineapp.PullJob{
				SourceID: source.ID,
				Command:  command,
				Config:   []byte(source.ConfigJSON),
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

	log.Printf("listening on %s", cfg.Addr)
	if err := srv.Start(); err != nil {
		log.Fatalf("http server: %v", err)
	}
}
