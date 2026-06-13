package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/csilservices"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	"github.com/catalystcommunity/longhouse/api/internal/recurrence"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"github.com/catalystcommunity/longhouse/api/internal/tcp"
	"github.com/rs/cors"
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
	if err := EnsureInitialTrustedDomain(); err != nil {
		return fmt.Errorf("failed to ensure initial trusted domain: %w", err)
	}

	// Recurrence worker.
	if !config.RecurrenceDisabled {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go runRecurrenceWorker(ctx, time.Duration(config.RecurrenceTickIntervalSec)*time.Second)
	} else {
		log.Info("Recurrence worker disabled")
	}

	// Notification cull worker — prunes feed items past the retention window.
	if !config.NotificationCullDisabled {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go runNotificationCullWorker(ctx,
			time.Duration(config.NotificationCullTickSeconds)*time.Second,
			time.Duration(config.NotificationRetentionDays)*24*time.Hour)
	} else {
		log.Info("Notification cull worker disabled")
	}

	// Trash purge worker — permanently deletes soft-deleted rows past the
	// retention window. The audit record of the delete outlives this.
	if !config.TrashPurgeDisabled {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go runTrashPurgeWorker(ctx,
			time.Duration(config.TrashPurgeTickSeconds)*time.Second,
			time.Duration(config.TrashRetentionDays)*24*time.Hour)
	} else {
		log.Info("Trash purge worker disabled")
	}

	// Audit partition maintenance — rolls the monthly audit_log partition
	// window forward and drops aged-out partitions (retention).
	if !config.AuditPartitionDisabled {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go runAuditPartitionWorker(ctx,
			time.Duration(config.AuditPartitionTickSeconds)*time.Second,
			config.AuditPartitionAheadMonths, config.AuditRetentionMonths)
	} else {
		log.Info("Audit partition worker disabled")
	}

	// TCP server (legacy CSIL-on-raw-TCP listener, kept until callers migrate).
	go func() {
		log.Infof("Starting TCP server on :%d", config.TCPPort)
		if err := tcp.ListenAndServe(fmt.Sprintf(":%d", config.TCPPort)); err != nil {
			log.WithError(err).Error("TCP server error")
		}
	}()

	handler, err := buildHTTPHandler()
	if err != nil {
		return err
	}
	log.Infof("Starting HTTP server on :%d", config.APIPort)
	return http.ListenAndServe(fmt.Sprintf(":%d", config.APIPort), handler)
}

// buildHTTPHandler assembles the public HTTP surface. The bulk of it lives
// at POST /api/csil/{service}/{method} via the CSIL-RPC dispatcher. Two
// small non-CSIL endpoints remain:
//
//	GET /api/health          — k8s probe, always 200 ok.
//	GET /api/v1/auth/start   — browser navigation that 302s to the IDP;
//	                           can't fit into the RPC POST pattern.
func buildHTTPHandler() (http.Handler, error) {
	if config.JWTSecret == "" {
		log.Warn("LONGHOUSE_JWT_SECRET is empty: every CSIL method will fail-closed with 'auth not configured'")
	}
	jwtSecret := []byte(config.JWTSecret)

	d := csilrpc.New(jwtSecret)
	// Wire the audit sink so the dispatcher records every authenticated
	// mutation (auth/devauth emit their own security events).
	d.SetAuditRecorder(store.AppStore)

	authSvc := buildAuthService()
	if authSvc != nil {
		authSvc.Register(d)
	}
	(&csilservices.HouseService{Store: store.AppStore}).Register(d)
	(&csilservices.MemberService{Store: store.AppStore}).Register(d)
	(&csilservices.RoleService{Store: store.AppStore}).Register(d)
	(&csilservices.GroupService{Store: store.AppStore}).Register(d)
	(&csilservices.SkillService{Store: store.AppStore}).Register(d)
	(&csilservices.TaskService{Store: store.AppStore}).Register(d)
	(&csilservices.EventService{Store: store.AppStore}).Register(d)
	(&csilservices.ProjectService{Store: store.AppStore}).Register(d)
	(&csilservices.DependencyService{Store: store.AppStore}).Register(d)
	(&csilservices.SettingsService{Store: store.AppStore}).Register(d)
	(&csilservices.BugService{Store: store.AppStore}).Register(d)
	(&csilservices.TrustedDomainService{Store: store.AppStore}).Register(d)
	(&csilservices.CommentService{Store: store.AppStore}).Register(d)
	(&csilservices.NotificationService{Store: store.AppStore}).Register(d)
	(&csilservices.AuditService{Store: store.AppStore}).Register(d)
	(&csilservices.TrashService{Store: store.AppStore}).Register(d)

	if devSvc := buildDevAuthService(authSvc); devSvc != nil {
		devSvc.Register(d)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", healthHandler)
	if authSvc != nil && authSvc.IDPURL != "" && authSvc.CallbackURL != "" {
		mux.Handle("GET /api/v1/auth/start", browserAuthStartHandler(jwtSecret, authSvc))
	} else {
		mux.Handle("GET /api/v1/auth/start", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "browser login is not configured on this server", http.StatusInternalServerError)
		}))
	}
	mux.Handle("/api/csil/", d)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})
	return c.Handler(mux), nil
}

// browserAuthStartHandler kicks off the linkkeys assertion exchange. The RP
// signs an auth request bound to the SPA's callback URL, then we 302 the
// browser to the IDP authorize page. The nonce round-trips inside the
// assertion and is re-checked at AuthService.Complete.
func browserAuthStartHandler(jwtSecret []byte, s *csilservices.AuthService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.PKI == nil || s.IDPURL == "" || s.CallbackURL == "" {
			http.Error(w, "browser login is not configured on this server", http.StatusInternalServerError)
			return
		}
		nonce := auth.MintNonce(jwtSecret)
		signedRequest, err := s.PKI.SignRequest(s.CallbackURL, nonce)
		if err != nil {
			log.WithError(err).Error("auth/start: RP sign-request failed")
			http.Error(w, "could not reach identity service", http.StatusBadGateway)
			return
		}
		q := url.Values{}
		q.Set("signed_request", signedRequest)
		http.Redirect(w, r, s.IDPURL+"/auth/authorize?"+q.Encode(), http.StatusFound)
	})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// buildAuthService wires the AuthService. JWTSecret is the hard requirement;
// without it we return nil and Login/Complete/Refresh/Me are absent from the
// dispatcher. PKI is optional — when missing, Login/Complete refuse with
// Internal but Refresh/Me still work for any caller holding a valid bearer
// minted via DevAuthService.
func buildAuthService() *csilservices.AuthService {
	if config.JWTSecret == "" {
		return nil
	}
	// RPDomain is the audience we expect on assertions. Prefer the explicit
	// LONGHOUSE_LINKKEYS_DOMAIN; in this single-IDP self-RP deployment it
	// equals the IDP domain, so fall back to that rather than silently
	// disabling the audience check when the RP domain is left unset.
	rpDomain := config.LinkkeysDomain
	if rpDomain == "" {
		rpDomain = config.LinkkeysIDPDomain
	}
	svc := &csilservices.AuthService{
		Store:       store.AppStore,
		JWTSecret:   []byte(config.JWTSecret),
		IDPDomain:   config.LinkkeysIDPDomain,
		RPDomain:    rpDomain,
		IDPURL:      config.LinkkeysIDPURL,
		CallbackURL: config.AppCallbackURL,
	}
	if config.LinkkeysPKIURL != "" && config.LinkkeysIDPDomain != "" {
		svc.PKI = linkkeys.New(
			config.LinkkeysPKIURL,
			config.LinkkeysPKIAPIKey,
			config.LinkkeysPKIAllowInvalid,
		)
	} else {
		log.Warn("linkkeys browser flow not configured: auth.Login/Complete will fail. " +
			"Use devauth.DevLogin locally (LONGHOUSE_DEV_AUTH_ENABLED=true).")
	}
	return svc
}

// buildDevAuthService is the gated counterpart. Returns nil unless every
// safety gate passes. The service shares the same AuthService instance so
// both code paths emit identical tokens.
func buildDevAuthService(authSvc *csilservices.AuthService) *csilservices.DevAuthService {
	if !config.DevAuthEnabled {
		return nil
	}
	if !config.DevAuthAllowed() {
		log.WithFields(log.Fields{
			"env":           config.Env,
			"dev_auth_flag": config.DevAuthEnabled,
		}).Warn("LONGHOUSE_DEV_AUTH_ENABLED=true ignored: LONGHOUSE_ENV is not dev/nonprod")
		return nil
	}
	if authSvc == nil {
		log.Warn("dev-auth requested but auth service is unconfigured; endpoints disabled")
		return nil
	}
	log.WithField("env", config.Env).Warn("DEV-AUTH ENABLED: devauth.DevLogin is reachable without assertion verification")
	return &csilservices.DevAuthService{Auth: authSvc, Env: config.Env}
}

// runRecurrenceWorker drives recurrence.Tick on a clock. Errors from a
// single tick are logged but don't kill the loop. Returns on ctx cancel.
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

// runTrashPurgeWorker permanently deletes soft-deleted rows whose deleted_at is
// older than the retention window — the trash bin's "empty after N days". The
// audit log entry for the original delete is NOT touched (audit retention is
// far longer), so "who deleted X" survives the purge. Errors are logged.
func runTrashPurgeWorker(ctx context.Context, interval, retention time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	if retention <= 0 {
		retention = 30 * 24 * time.Hour
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	log.Infof("Trash purge worker tick = %s, retention = %s", interval, retention)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			cutoff := now.UTC().Add(-retention)
			n, err := store.AppStore.PurgeAllSoftDeletedBefore(ctx, cutoff)
			if err != nil {
				log.WithError(err).Error("trash purge failed")
				continue
			}
			if n > 0 {
				log.WithFields(log.Fields{"purged_rows": n, "cutoff": cutoff}).Info("trash purge")
			}
		}
	}
}

// runAuditPartitionWorker keeps the audit_log partition window healthy: it
// pre-creates `ahead` months of partitions and drops partitions older than
// `retentionMonths`. Runs once immediately so a fresh deploy has its window
// before the first tick. Errors are logged, not fatal.
func runAuditPartitionWorker(ctx context.Context, interval time.Duration, ahead, retentionMonths int) {
	if interval <= 0 {
		interval = time.Hour
	}
	if ahead < 0 {
		ahead = 2
	}
	if retentionMonths <= 0 {
		retentionMonths = 24
	}
	maintain := func(now time.Time) {
		if err := store.AppStore.CreateAuditPartitionsAhead(ctx, now, ahead); err != nil {
			log.WithError(err).Error("audit partition create failed")
		}
		dropped, err := store.AppStore.DropAuditPartitionsBefore(ctx, now, retentionMonths)
		if err != nil {
			log.WithError(err).Error("audit partition drop failed")
		} else if dropped > 0 {
			log.WithFields(log.Fields{"dropped_partitions": dropped}).Info("audit partition retention")
		}
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	log.Infof("Audit partition worker tick = %s, ahead = %d months, retention = %d months", interval, ahead, retentionMonths)
	maintain(time.Now().UTC())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			maintain(now.UTC())
		}
	}
}

// runNotificationCullWorker periodically deletes notification events (and
// their cascaded per-recipient rows) older than the retention window, so the
// feed prunes itself. Errors from a single sweep are logged, not fatal.
func runNotificationCullWorker(ctx context.Context, interval, retention time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	if retention <= 0 {
		retention = 180 * 24 * time.Hour
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	log.Infof("Notification cull worker tick = %s, retention = %s", interval, retention)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			cutoff := now.UTC().Add(-retention)
			n, err := store.AppStore.CullNotificationEventsBefore(ctx, cutoff)
			if err != nil {
				log.WithError(err).Error("notification cull failed")
				continue
			}
			if n > 0 {
				log.WithFields(log.Fields{"culled_events": n, "cutoff": cutoff}).Info("notification cull")
			}
		}
	}
}

// recurrenceStore adapts the global store.AppStore to the small
// recurrence.WorkerStore interface so the worker package doesn't need to
// import the whole Store surface.
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
func (recurrenceStore) ListTaskAssignees(ctx context.Context, taskID string) ([]models.Member, error) {
	return store.AppStore.ListTaskAssignees(ctx, taskID)
}
func (recurrenceStore) AddTaskAssignee(ctx context.Context, taskID, memberID string) error {
	return store.AppStore.AddTaskAssignee(ctx, taskID, memberID)
}
func (recurrenceStore) ListDueRecurringEvents(ctx context.Context, before time.Time, limit int) ([]models.Event, error) {
	return store.AppStore.ListDueRecurringEvents(ctx, before, limit)
}
func (recurrenceStore) LatestRecurrenceChildOfEvent(ctx context.Context, rootEventID string) (*models.Event, error) {
	return store.AppStore.LatestRecurrenceChildOfEvent(ctx, rootEventID)
}
func (recurrenceStore) CreateEvent(ctx context.Context, event *models.Event) error {
	return store.AppStore.CreateEvent(ctx, event)
}
func (recurrenceStore) UpdateEvent(ctx context.Context, event *models.Event) error {
	return store.AppStore.UpdateEvent(ctx, event)
}
