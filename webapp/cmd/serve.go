package cmd

import (
	"fmt"
	"net/http"

	"github.com/catalystcommunity/longhouse/webapp/internal/config"
	"github.com/catalystcommunity/longhouse/webapp/internal/handlers"
	log "github.com/sirupsen/logrus"
)

func Serve(flags map[string]string) error {
	config.ApplyFlags(flags)

	if config.SessionSecret == "" {
		return fmt.Errorf("LONGHOUSE_SESSION_SECRET must be set")
	}

	handler := handlers.NewRouter(nil)
	log.Infof("Starting web UI on :%d (API: %s)", config.WebPort, config.APIURL)
	return http.ListenAndServe(fmt.Sprintf(":%d", config.WebPort), handler)
}
