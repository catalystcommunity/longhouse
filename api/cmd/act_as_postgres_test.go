//go:build integration

package cmd

import (
	"bytes"
	"context"
	"github.com/catalystcommunity/longhouse/api/internal/actas"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/csilservices"
	"github.com/catalystcommunity/longhouse/api/internal/transport"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres"
)

func TestActAsNonce_Postgres_ConcurrentAndPersistent(t *testing.T) {
	freshStore(t, requireTestDB(t))
	ctx := context.Background()
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := store.AppStore.ConsumeActAsNonce(ctx, "home.test", "grant", "nonce", time.Now().Add(time.Hour))
			if err != nil {
				t.Error(err)
			}
			if ok {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted %d concurrent replays", accepted.Load())
	}
	// A new store object has no local nonce state.
	another := &postgres.PostgresStore{}
	if ok, err := another.ConsumeActAsNonce(ctx, "home.test", "grant", "nonce", time.Now().Add(time.Hour)); err != nil || ok {
		t.Fatalf("replay after store replacement: %v %v", ok, err)
	}
	for _, pair := range [][2]string{{"other.test", "grant"}, {"home.test", "other"}} {
		if ok, err := another.ConsumeActAsNonce(ctx, pair[0], pair[1], "nonce", time.Now().Add(time.Hour)); err != nil || !ok {
			t.Fatalf("separate grant denied: %v %v", ok, err)
		}
	}
	if ok, err := another.ConsumeActAsNonce(ctx, "home.test", "expired", "nonce", time.Now().Add(-time.Second)); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := another.ConsumeActAsNonce(ctx, "home.test", "expired", "nonce", time.Now().Add(time.Hour)); err != nil || !ok {
		t.Fatal("expired nonce was not removed", err)
	}
}

type integrationGrant struct{ revoked bool }

func (p *integrationGrant) SignScopeSet(context.Context, actas.ScopeSetRequest) ([]byte, error) {
	return nil, nil
}
func (p *integrationGrant) VerifyCredential(context.Context, []byte, []string) (actas.Verified, error) {
	return actas.Verified{GrantID: "grant", SubjectDomain: "soft.test", SubjectUserID: "person", Grantee: "owner@app.test/helper", Scopes: []string{"longhouse.full"}, ExpiresAt: time.Now().Add(time.Hour), Nonce: "nonce"}, nil
}
func (p *integrationGrant) RevokedGrants(context.Context, string, []string) ([]string, error) {
	if p.revoked {
		return []string{"grant"}, nil
	}
	return nil, nil
}
func (p *integrationGrant) MaxPresentationAge() time.Duration { return 7 * time.Minute }

func TestActAs_Postgres_DelegatedTasksProjectsAndRevocation(t *testing.T) {
	freshStore(t, requireTestDB(t))
	ctx := context.Background()
	house := mkHouse(t, ctx, "Delegated house")
	member := mkMember(t, ctx, house, "person")
	provider := &integrationGrant{}
	service := &csilservices.AuthService{Store: store.AppStore, JWTSecret: []byte("test"), ActAs: provider}
	d := csilrpc.New(service.JWTSecret)
	d.SetAuditRecorder(store.AppStore)
	service.Register(d)
	(&csilservices.TaskService{Store: store.AppStore}).Register(d)
	(&csilservices.ProjectService{Store: store.AppStore}).Register(d)
	(&csilservices.HouseService{Store: store.AppStore}).Register(d)
	call := func(service, op string, payload []byte, token string) transport.RpcResponse {
		t.Helper()
		request := transport.NewRpcRequest(service, op, payload)
		body, err := request.Encode()
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/csil/v1/rpc", bytes.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		d.ServeRPC(response, req)
		result, err := transport.DecodeRpcResponse(response.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	requireOK := func(r transport.RpcResponse) {
		t.Helper()
		if !r.Status.IsOk() || r.Variant == nil || *r.Variant == "ServiceError" {
			t.Fatalf("RPC failed: %+v", r)
		}
	}
	exchanged := call("AuthService", "exchange-act-as-credential", csil.EncodeExchangeActAsCredentialRequest(csil.ExchangeActAsCredentialRequest{Credential: []byte{1}}), "")
	requireOK(exchanged)
	session, err := csil.DecodeActAsTokenResponse(exchanged.Payload)
	if err != nil {
		t.Fatal(err)
	}
	projectResponse := call("ProjectService", "create-project", csil.EncodeProject(csil.Project{HouseId: csil.HouseID(house), Name: "Delegated project"}), session.Token)
	requireOK(projectResponse)
	project, err := csil.DecodeProject(projectResponse.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if project.ProjectId == "" {
		t.Fatal("missing project")
	}
	taskResponse := call("TaskService", "create-personal-task", csil.EncodeTask(csil.Task{HouseId: csil.HouseID(house), Title: "Delegated task"}), session.Token)
	requireOK(taskResponse)
	task, err := csil.DecodeTask(taskResponse.Payload)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := store.AppStore.GetTaskByID(ctx, string(task.TaskId))
	if err != nil {
		t.Fatal(err)
	}
	if saved.OwnerMemberID != member {
		t.Fatalf("task not attributed to principal: %+v", saved)
	}
	// Full scope does not grant the user's missing administrator role.
	denied := call("HouseService", "delete-house", csil.EncodeHouseDeleteHouseRequest(csil.HouseID(house)), session.Token)
	failure, err := csil.DecodeServiceError(denied.Payload)
	if err != nil || failure.Code != 403 {
		t.Fatalf("role escalation: %+v %v", failure, err)
	}
	provider.revoked = true
	denied = call("ProjectService", "create-project", csil.EncodeProject(csil.Project{HouseId: csil.HouseID(house), Name: "Must not exist"}), session.Token)
	failure, err = csil.DecodeServiceError(denied.Payload)
	if err != nil || failure.Code != 401 {
		t.Fatalf("revoked bearer accepted: %+v %v", failure, err)
	}
}
