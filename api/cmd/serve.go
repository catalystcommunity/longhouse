package cmd

import (
	"fmt"
	"net/http"

	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/api/internal/handlers"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres"
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

	// Start TCP server in background
	go func() {
		log.Infof("Starting TCP server on :%d", config.TCPPort)
		if err := tcp.ListenAndServe(fmt.Sprintf(":%d", config.TCPPort)); err != nil {
			log.WithError(err).Error("TCP server error")
		}
	}()

	// Start HTTP server
	handler := handlers.NewRouter()
	log.Infof("Starting HTTP server on :%d", config.APIPort)
	return http.ListenAndServe(fmt.Sprintf(":%d", config.APIPort), handler)
}
