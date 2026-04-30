package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/api/internal/handlers"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/recurrence"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"github.com/catalystcommunity/longhouse/api/internal/tcp"
	log "github.com/sirupsen/logrus"
)

func Serve(flags map[string]string) error {
	config.ApplyFlags(flags)

	// Run migrations
	if err := RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize store
	store.AppStore = &postgres.PostgresStore{}
	cleanup, err := store.AppStore.Initialize()
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	log.Info("Store initialized")

	if err := SeedInitialAdmin(); err != nil {
		return fmt.Errorf("failed to seed initial admin: %w", err)
	}

	// Start the recurrence worker in the background unless explicitly
	// disabled. A separate context lets the goroutine shut down cleanly
	// if the http server returns; in practice ListenAndServe blocks
	// forever, so this is mostly defense for graceful-stop additions.
	if !config.RecurrenceDisabled {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go runRecurrenceWorker(ctx, time.Duration(config.RecurrenceTickIntervalSec)*time.Second)
	} else {
		log.Info("Recurrence worker disabled")
	}

	// Start TCP server in background
	go func() {
		log.Infof("Starting TCP server on :%d", config.TCPPort)
		if err := tcp.ListenAndServe(fmt.Sprintf(":%d", config.TCPPort)); err != nil {
			log.WithError(err).Error("TCP server error")
		}
	}()

	// Start HTTP server
	deps := &handlers.RouterDeps{Auth: buildAuthDeps()}
	handler := handlers.NewRouter(deps)
	log.Infof("Starting HTTP server on :%d", config.APIPort)
	return http.ListenAndServe(fmt.Sprintf(":%d", config.APIPort), handler)
}

// runRecurrenceWorker drives recurrence.Tick on a clock. Errors from a
// single tick are logged but don't kill the loop — the next tick gets a
// fresh shot. Returns on ctx cancellation.
func runRecurrenceWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	log.Infof("Recurrence worker tick = %s", interval)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			res, err := recurrence.Tick(ctx, recurrenceStore{}, now.UTC())
			if err != nil {
				log.WithError(err).Error("recurrence tick failed")
				continue
			}
			if res.Spawned > 0 || res.MissedComments > 0 || len(res.Errors) > 0 {
				log.WithFields(log.Fields{
					"considered":      res.Considered,
					"spawned":         res.Spawned,
					"missed_comments": res.MissedComments,
					"errors":          len(res.Errors),
				}).Info("recurrence tick")
			}
		}
	}
}

// recurrenceStore adapts the global store.AppStore to the small
// recurrence.WorkerStore interface so the worker package doesn't need to
// import the whole Store surface. The concrete *postgres.PostgresStore
// already implements every method on the worker interface — the adapter
// here is mostly explicit for readability.
type recurrenceStore struct{}

func (recurrenceStore) ListDueRecurringTasks(ctx context.Context, before time.Time, limit int) ([]models.Task, error) {
	return store.AppStore.ListDueRecurringTasks(ctx, before, limit)
}
func (recurrenceStore) LatestRecurrenceChildOf(ctx context.Context, rootTaskID string) (*models.Task, error) {
	return store.AppStore.LatestRecurrenceChildOf(ctx, rootTaskID)
}
func (recurrenceStore) CreateTask(ctx context.Context, task *models.Task) error {
	return store.AppStore.CreateTask(ctx, task)
}
func (recurrenceStore) UpdateTask(ctx context.Context, task *models.Task) error {
	return store.AppStore.UpdateTask(ctx, task)
}
func (recurrenceStore) CreateComment(ctx context.Context, comment *models.Comment) error {
	return store.AppStore.CreateComment(ctx, comment)
}

// buildAuthDeps wires the AuthDeps when configuration is present. When the
// linkkeys + JWT envs aren't set we leave it nil — /auth/login + /me are
// then absent from the router so the api fails closed instead of issuing
// tokens against an unset secret.
func buildAuthDeps() *handlers.AuthDeps {
	if config.JWTSecret == "" || config.LinkkeysPKIURL == "" || config.LinkkeysIDPDomain == "" {
		log.Warn("Auth not fully configured: /api/v1/auth/login and /me will be unavailable")
		return nil
	}
	return &handlers.AuthDeps{
		PKI: linkkeys.New(
			config.LinkkeysPKIURL,
			config.LinkkeysPKIAPIKey,
			config.LinkkeysPKIAllowInvalid,
		),
		Store:     store.AppStore,
		IDPDomain: config.LinkkeysIDPDomain,
		JWTSecret: []byte(config.JWTSecret),
	}
}
