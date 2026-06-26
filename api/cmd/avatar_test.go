package cmd

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/avatar"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// avatarFakeStore implements just the three methods the handler touches.
type avatarFakeStore struct {
	store.Store
	member *models.Member
	cached *models.CachedAvatar
	put    *models.CachedAvatar
}

func (s *avatarFakeStore) GetMemberByID(context.Context, string) (*models.Member, error) {
	if s.member == nil {
		return nil, nil
	}
	return s.member, nil
}
func (s *avatarFakeStore) GetCachedAvatar(context.Context, string) (*models.CachedAvatar, error) {
	return s.cached, nil
}
func (s *avatarFakeStore) PutCachedAvatar(_ context.Context, a *models.CachedAvatar) error {
	s.put = a
	return nil
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const avatarSecret = "avatar-test-secret"

func bearerFor(t *testing.T, houseID string) string {
	t.Helper()
	tok, err := auth.Mint([]byte(avatarSecret), auth.Identity{
		Domain: "d", UserID: "u",
		Houses: []auth.HouseRoles{{House: houseID, Member: "m1", Roles: []string{"member"}}},
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func newTestAvatarHandler(st store.Store) *avatarHandler {
	// AllowLoopback so the fetch path can reach httptest (127.0.0.1).
	return &avatarHandler{store: st, fetcher: avatar.New(avatar.Options{AllowLoopback: true}), jwtSecret: []byte(avatarSecret)}
}

func doReq(h http.Handler, token, memberID string, ifNoneMatch string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/v1/avatars/"+memberID, nil)
	req.SetPathValue("member", memberID)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestAvatarHandler_RequiresBearer(t *testing.T) {
	h := newTestAvatarHandler(&avatarFakeStore{})
	if rr := doReq(h, "", "m1", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no bearer = %d, want 401", rr.Code)
	}
}

func TestAvatarHandler_404OutsideHouse(t *testing.T) {
	st := &avatarFakeStore{member: &models.Member{MemberID: "m1", HouseID: "hX", AvatarURL: "https://x/a.png"}}
	h := newTestAvatarHandler(st)
	// Bearer carries a different house → must not reveal the avatar.
	if rr := doReq(h, bearerFor(t, "hOther"), "m1", ""); rr.Code != http.StatusNotFound {
		t.Fatalf("cross-house = %d, want 404", rr.Code)
	}
}

func TestAvatarHandler_CacheHit(t *testing.T) {
	st := &avatarFakeStore{
		member: &models.Member{MemberID: "m1", HouseID: "hX", AvatarURL: "https://x/a.png"},
		cached: &models.CachedAvatar{ContentType: "image/png", Bytes: testPNG(t, 32, 32)},
	}
	h := newTestAvatarHandler(st)
	rr := doReq(h, bearerFor(t, "hX"), "m1", "")
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("cache hit = %d ct=%q", rr.Code, rr.Header().Get("Content-Type"))
	}
	if rr.Header().Get("ETag") == "" {
		t.Fatal("expected an ETag on a cached response")
	}
	if st.put != nil {
		t.Fatal("cache hit must not re-fetch/store")
	}
}

func TestAvatarHandler_LazyFetchOnMiss(t *testing.T) {
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(testPNG(t, 48, 48))
	}))
	defer src.Close()
	st := &avatarFakeStore{member: &models.Member{MemberID: "m1", HouseID: "hX", AvatarURL: src.URL}}
	h := newTestAvatarHandler(st)
	rr := doReq(h, bearerFor(t, "hX"), "m1", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("lazy fetch = %d, want 200", rr.Code)
	}
	if st.put == nil || st.put.ContentType != "image/png" || st.put.Width != 48 {
		t.Fatalf("expected fetched image cached, got %+v", st.put)
	}
}

func TestAvatarHandler_FetchFailure404(t *testing.T) {
	// Private/unreachable URL → SSRF guard or fetch error → 404 (initials fallback).
	st := &avatarFakeStore{member: &models.Member{MemberID: "m1", HouseID: "hX", AvatarURL: "http://10.0.0.1/a.png"}}
	h := &avatarHandler{store: st, fetcher: avatar.New(avatar.Options{}), jwtSecret: []byte(avatarSecret)} // strict guard
	if rr := doReq(h, bearerFor(t, "hX"), "m1", ""); rr.Code != http.StatusNotFound {
		t.Fatalf("fetch failure = %d, want 404", rr.Code)
	}
}

func TestAvatarHandler_NotModified(t *testing.T) {
	body := testPNG(t, 32, 32)
	st := &avatarFakeStore{
		member: &models.Member{MemberID: "m1", HouseID: "hX", AvatarURL: "https://x/a.png"},
		cached: &models.CachedAvatar{ContentType: "image/png", Bytes: body},
	}
	h := newTestAvatarHandler(st)
	// First request to learn the ETag, then replay it.
	first := doReq(h, bearerFor(t, "hX"), "m1", "")
	etag := first.Header().Get("ETag")
	if rr := doReq(h, bearerFor(t, "hX"), "m1", etag); rr.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match = %d, want 304", rr.Code)
	}
}
