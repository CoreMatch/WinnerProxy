package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/winnerproxy/winnerproxy/internal/cache"
	"github.com/winnerproxy/winnerproxy/internal/handler"
	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
	"github.com/winnerproxy/winnerproxy/internal/proxy"
	"github.com/winnerproxy/winnerproxy/internal/router"
)

// -----------------------------------------------------------------------------
// fakes
// -----------------------------------------------------------------------------

// fakeMojang is a stub UpstreamService for the stage-2 fallback path.
// All methods can be overridden per-test; ID() always returns "mojang"
// so handler.findMojang picks it up.
type fakeMojang struct {
	hasJoinedFn    func(p url.Values) (*hrpauth.PlayerProfile, error)
	queryProfileFn func(uuid string, unsigned bool) (*hrpauth.PlayerProfile, error)
	batchQueryFn   func(names []string) ([]*hrpauth.PlayerProfile, error)
}

func (f *fakeMojang) ID() string { return "mojang" }
func (f *fakeMojang) HasJoined(p url.Values) (*hrpauth.PlayerProfile, error) {
	if f.hasJoinedFn == nil {
		return nil, hrpauth.ErrNoProfile
	}
	return f.hasJoinedFn(p)
}
func (f *fakeMojang) QueryProfile(uuid string, unsigned bool) (*hrpauth.PlayerProfile, error) {
	if f.queryProfileFn == nil {
		return nil, hrpauth.ErrNoProfile
	}
	return f.queryProfileFn(uuid, unsigned)
}
func (f *fakeMojang) BatchQuery(names []string) ([]*hrpauth.PlayerProfile, error) {
	if f.batchQueryFn == nil {
		return nil, nil
	}
	return f.batchQueryFn(names)
}

// fakeHA is a programmable httptest server backing the HRPAuth client.
// Each route is dispatched to a per-field http.HandlerFunc; defaults
// are recorded so tests can assert what was called.
type fakeHA struct {
	hasJoin  http.HandlerFunc
	getProf  http.HandlerFunc
	batch    http.HandlerFunc
	register http.HandlerFunc
	root     http.HandlerFunc

	hasJoinCalls  atomic.Int32
	getProfCalls  atomic.Int32
	batchCalls    atomic.Int32
	registerCalls atomic.Int32
	rootCalls     atomic.Int32
}

func newFakeHA(t *testing.T) (*fakeHA, *httptest.Server) {
	t.Helper()
	ha := &fakeHA{}
	mux := http.NewServeMux()
	mux.HandleFunc("/sessionserver/session/minecraft/hasJoined", func(w http.ResponseWriter, r *http.Request) {
		ha.hasJoinCalls.Add(1)
		if ha.hasJoin != nil {
			ha.hasJoin(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/sessionserver/session/minecraft/profile/", func(w http.ResponseWriter, r *http.Request) {
		ha.getProfCalls.Add(1)
		if ha.getProf != nil {
			ha.getProf(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/api/profiles/minecraft", func(w http.ResponseWriter, r *http.Request) {
		ha.batchCalls.Add(1)
		if ha.batch != nil {
			ha.batch(w, r)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		ha.registerCalls.Add(1)
		if ha.register != nil {
			ha.register(w, r)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		ha.rootCalls.Add(1)
		if ha.root != nil {
			ha.root(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"skinDomains":[]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return ha, srv
}

// buildRouter wires the handler with the given mojang service and
// profile cache against the supplied fakeHA server.
func buildRouter(t *testing.T, srv *httptest.Server, mojang proxy.UpstreamService, c cache.ProfileCache) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cli := hrpauth.New(srv.URL, "test-mt", nil)
	var services []proxy.UpstreamService
	if mojang != nil {
		services = append(services, mojang)
	}
	h := handler.New(services, cli, c)
	return router.New(h)
}

// doRequest runs one HTTP call against the router and returns the
// recorded response.
func doRequest(eng *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var rdr *bytes.Reader
	if body != "" {
		rdr = bytes.NewReader([]byte(body))
	}
	var req *http.Request
	if rdr != nil {
		req = httptest.NewRequest(method, path, rdr)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	eng.ServeHTTP(w, req)
	return w
}

// newCache returns a small in-memory profile cache for tests.
func newCache() *cache.FreeCache {
	return cache.NewFreeCache(1<<20, 60_000_000_000) // 1 MiB, 60s TTL
}

// hasJoinedURL returns a sample hasJoined request path.
func hasJoinedURL(name string) string {
	return "/yggdrasil/sessionserver/session/minecraft/hasJoined?username=" +
		name + "&serverId=test"
}

// -----------------------------------------------------------------------------
// HasJoined — three-stage flow
// -----------------------------------------------------------------------------

// TestHasJoinedStage1Hit exercises the happy path where HRPAuth
// hasJoined returns 200. The handler must return the HA profile
// unchanged and warm the cache for later QueryProfile calls.
func TestHasJoinedStage1Hit(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(hrpauth.PlayerProfile{
			ID:   "ha-uuid-1",
			Name: "alice",
			Properties: []hrpauth.PlayerProperty{{
				Name: "textures", Value: "ha-skin", Signature: "sig",
			}},
		})
	}
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		t.Fatalf("mojang should not be called on HA hit")
		return nil, nil
	}}
	c := newCache()
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("alice"), "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
	var p hrpauth.PlayerProfile
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.ID != "ha-uuid-1" || p.Name != "alice" {
		t.Fatalf("unexpected profile: %+v", p)
	}
	if cached, ok := c.GetHAProfile("ha-uuid-1"); !ok || cached.Name != "alice" {
		t.Fatalf("expected cache to be warmed, got ok=%v p=%+v", ok, cached)
	}
}

// TestHasJoinedStage2CacheHit checks the cache short-circuit: when
// the Mojang profile for a username is already cached, the handler
// must skip the upstream Mojang call but still invoke the HA
// /register endpoint.
func TestHasJoinedStage2CacheHit(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	var mjCalled atomic.Int32
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		mjCalled.Add(1)
		return nil, hrpauth.ErrNoProfile
	}}
	ha.register = func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["mojang_uuid"] != "moj-uuid-1" {
			t.Errorf("expected mojang_uuid=moj-uuid-1, got %v", body["mojang_uuid"])
		}
		_ = json.NewEncoder(w).Encode(hrpauth.RegisterResponse{
			Success:   true,
			ProfileID: "ha-profile-id-9",
		})
	}
	c := newCache()
	c.SetMojangProfile("alice", &hrpauth.PlayerProfile{
		ID:   "moj-uuid-1",
		Name: "alice",
		Properties: []hrpauth.PlayerProperty{{
			Name: "textures", Value: "moj-skin", Signature: "sig",
		}},
	})
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("alice"), "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
	var p hrpauth.PlayerProfile
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.ID != "ha-profile-id-9" {
		t.Errorf("profile_id: got %q want ha-profile-id-9", p.ID)
	}
	if p.Name != "alice" {
		t.Errorf("name: got %q want alice", p.Name)
	}
	if len(p.Properties) != 1 || p.Properties[0].Value != "moj-skin" {
		t.Errorf("expected mojang skin passthrough, got %+v", p.Properties)
	}
	if mjCalled.Load() != 0 {
		t.Errorf("mojang should not be called on cache hit, got %d", mjCalled.Load())
	}
}

// TestHasJoinedRegisterUsernameBound covers the case where HA rejects
// the proxy-registration because the username is held by a non-
// bindable account. The handler must return 204.
func TestHasJoinedRegisterUsernameBound(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		return &hrpauth.PlayerProfile{ID: "moj-uuid-1", Name: "alice"}, nil
	}}
	ha.register = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "username_already_bound"})
	}
	c := newCache()
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("alice"), "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status: %d, want 204", w.Code)
	}
}

// TestHasJoinedRegisterUpstreamError covers HA's /register returning
// 5xx. Handler must return 503.
func TestHasJoinedRegisterUpstreamError(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		return &hrpauth.PlayerProfile{ID: "moj-uuid-1", Name: "alice"}, nil
	}}
	ha.register = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	c := newCache()
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("alice"), "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d, want 503, body=%s", w.Code, w.Body.String())
	}
}

// TestHasJoinedMojangNotFound covers HA 204 + Mojang 204. Handler
// must return 204.
func TestHasJoinedMojangNotFound(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		return nil, hrpauth.ErrNoProfile
	}}
	c := newCache()
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("nobody"), "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status: %d, want 204", w.Code)
	}
}

// TestHasJoinedStage1ErrorStage2Success checks that a 5xx from HA
// does not abort the request; the handler falls through to Mojang.
func TestHasJoinedStage1ErrorStage2Success(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		return &hrpauth.PlayerProfile{ID: "moj-uuid-1", Name: "bob"}, nil
	}}
	ha.register = func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(hrpauth.RegisterResponse{ProfileID: "ha-pid-1"})
	}
	c := newCache()
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("bob"), "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
}

// TestHasJoinedStage1EmptyBodyStage2Success covers HA's "not found"
// shape: 200 OK with body `{}`. The handler must treat this as a
// miss, fall through to the Mojang stage, and ultimately return the
// proxy-registered profile (HA identity + Mojang skin). It must NOT
// return the zero-value profile to the game server.
func TestHasJoinedStage1EmptyBodyStage2Success(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
	}
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		return &hrpauth.PlayerProfile{
			ID:   "moj-uuid-1",
			Name: "carol",
			Properties: []hrpauth.PlayerProperty{
				{Name: "textures", Value: "moj-skin", Signature: "sig"},
			},
		}, nil
	}}
	ha.register = func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(hrpauth.RegisterResponse{ProfileID: "ha-pid-empty"})
	}
	c := newCache()
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("carol"), "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var p hrpauth.PlayerProfile
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.ID != "ha-pid-empty" {
		t.Errorf("id: got %q want ha-pid-empty", p.ID)
	}
	if p.Name != "carol" {
		t.Errorf("name: got %q want carol", p.Name)
	}
	if len(p.Properties) != 1 || p.Properties[0].Value != "moj-skin" {
		t.Errorf("expected mojang skin passthrough, got %+v", p.Properties)
	}
}

// TestHasJoinedMojangDisabled covers Mojang disabled in config (no
// fakeMojang wired) + HA 204. Handler must return 204.
func TestHasJoinedMojangDisabled(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	c := newCache()
	eng := buildRouter(t, srv, nil, c) // no mojang service

	w := doRequest(eng, http.MethodGet, hasJoinedURL("x"), "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status: %d, want 204", w.Code)
	}
}

// -----------------------------------------------------------------------------
// QueryProfile / BatchQuery / YggdrasilRoot
// -----------------------------------------------------------------------------

// TestQueryProfileCacheHit verifies that a cached HA profile is
// returned without contacting the upstream.
func TestQueryProfileCacheHit(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.getProf = func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("hrpauth.GetProfile should not be called on cache hit")
	}
	c := newCache()
	c.SetHAProfile("cached-uuid", &hrpauth.PlayerProfile{
		ID: "cached-uuid", Name: "alice",
	})
	eng := buildRouter(t, srv, &fakeMojang{}, c)

	w := doRequest(eng, http.MethodGet,
		"/yggdrasil/sessionserver/session/minecraft/profile/cached-uuid?unsigned=true", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var p hrpauth.PlayerProfile
	_ = json.NewDecoder(w.Body).Decode(&p)
	if p.ID != "cached-uuid" {
		t.Fatalf("id: %s", p.ID)
	}
	if ha.getProfCalls.Load() != 0 {
		t.Errorf("expected 0 HRPAuth GetProfile calls, got %d", ha.getProfCalls.Load())
	}
}

// TestQueryProfileCacheMissThenHit verifies that on a miss the
// handler queries HA and warms the cache; the next call must hit.
func TestQueryProfileCacheMissThenHit(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.getProf = func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(hrpauth.PlayerProfile{ID: "uuid-1", Name: "alice"})
	}
	c := newCache()
	eng := buildRouter(t, srv, &fakeMojang{}, c)

	for i := 0; i < 2; i++ {
		w := doRequest(eng, http.MethodGet,
			"/yggdrasil/sessionserver/session/minecraft/profile/uuid-1?unsigned=true", "")
		if w.Code != http.StatusOK {
			t.Fatalf("iter %d: status %d", i, w.Code)
		}
	}
	if ha.getProfCalls.Load() != 1 {
		t.Errorf("expected HA to be called once, got %d", ha.getProfCalls.Load())
	}
}

// TestQueryProfileNotFound returns 404 when HA has no such profile.
func TestQueryProfileNotFound(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.getProf = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}
	c := newCache()
	eng := buildRouter(t, srv, &fakeMojang{}, c)

	w := doRequest(eng, http.MethodGet,
		"/yggdrasil/sessionserver/session/minecraft/profile/ghost?unsigned=true", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d, want 404", w.Code)
	}
}

// TestBatchQuery verifies the {id, name}-only summary.
func TestBatchQuery(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.batch = func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]*hrpauth.PlayerProfile{
			{ID: "u1", Name: "alice"},
			{ID: "u2", Name: "bob"},
		})
	}
	c := newCache()
	eng := buildRouter(t, srv, &fakeMojang{}, c)

	body := `["alice","bob"]`
	w := doRequest(eng, http.MethodPost,
		"/yggdrasil/api/profiles/minecraft", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"id":"u1"`) {
		t.Errorf("missing u1 in %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "properties") {
		t.Errorf("batch should not return properties, got %s", w.Body.String())
	}
}

// TestYggdrasilRootPassThrough checks that the meta response is
// transparently proxied.
func TestYggdrasilRootPassThrough(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.root = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"skinDomains":["a.example"],"signaturePublickey":"K"}`))
	}
	c := newCache()
	eng := buildRouter(t, srv, &fakeMojang{}, c)

	w := doRequest(eng, http.MethodGet, "/yggdrasil", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "a.example") {
		t.Errorf("missing skinDomains, got %s", w.Body.String())
	}
}

// TestYggdrasilRootFallback checks that an HA failure still yields
// a valid (if minimal) meta response.
func TestYggdrasilRootFallback(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.root = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	c := newCache()
	eng := buildRouter(t, srv, &fakeMojang{}, c)

	w := doRequest(eng, http.MethodGet, "/yggdrasil", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "skinDomains") {
		t.Errorf("missing fallback, got %s", w.Body.String())
	}
}

// -----------------------------------------------------------------------------
// Health
// -----------------------------------------------------------------------------

// TestHealth is a trivial smoke test.
func TestHealth(t *testing.T) {
	_, srv := newFakeHA(t)
	c := newCache()
	eng := buildRouter(t, srv, &fakeMojang{}, c)
	w := doRequest(eng, http.MethodGet, "/health", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}
