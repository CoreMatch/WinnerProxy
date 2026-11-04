package handler_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
)

// This file completes the hasJoined three-stage failure matrix. The
// pre-existing tests in handler_test.go already cover: stage-1 hit,
// stage-2 cache hit, register username_already_bound (204), register
// 5xx (503), HA 204 + Mojang miss (204), HA 5xx fallthrough to a
// successful stage 3, HA 200+{} fallthrough, and Mojang disabled.

// TestHasJoinedCleanThreeStageSuccess covers the canonical path with a
// clean stage-1 miss: HA 204 (no profile), Mojang hit, proxy-register
// success. The response carries the HA profile_id with the Mojang skin
// properties; the query is passed through verbatim to HA and the Mojang
// profile is cached for subsequent attempts.
func TestHasJoinedCleanThreeStageSuccess(t *testing.T) {
	ha, srv := newFakeHA(t)
	var gotUsername, gotServerID atomic.Value
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		gotUsername.Store(r.URL.Query().Get("username"))
		gotServerID.Store(r.URL.Query().Get("serverId"))
		w.WriteHeader(http.StatusNoContent)
	}
	mj := &fakeMojang{hasJoinedFn: func(q url.Values) (*hrpauth.PlayerProfile, error) {
		return &hrpauth.PlayerProfile{
			ID:   "moj-uuid-9",
			Name: "dave",
			Properties: []hrpauth.PlayerProperty{
				{Name: "textures", Value: "moj-skin", Signature: "sig"},
			},
		}, nil
	}}
	ha.register = func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["username"] != "dave" || body["mojang_uuid"] != "moj-uuid-9" {
			t.Errorf("unexpected register body: %v", body)
		}
		_ = json.NewEncoder(w).Encode(hrpauth.RegisterResponse{ProfileID: "ha-pid-77"})
	}
	c := newCache()
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("dave"), "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var p hrpauth.PlayerProfile
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.ID != "ha-pid-77" || p.Name != "dave" {
		t.Fatalf("unexpected profile: %+v", p)
	}
	if len(p.Properties) != 1 || p.Properties[0].Value != "moj-skin" {
		t.Errorf("expected Mojang skin passthrough, got %+v", p.Properties)
	}
	if gotUsername.Load() != "dave" || gotServerID.Load() != "test" {
		t.Errorf("query not passed through verbatim: username=%v serverId=%v",
			gotUsername.Load(), gotServerID.Load())
	}
	if _, ok := c.GetMojangProfile("dave"); !ok {
		t.Error("expected the positive Mojang answer to be cached")
	}
}

// TestHasJoinedHA5xxMojangDisabled covers the exhausted fallback chain:
// HA fails and the Mojang upstream is disabled. The join must fail
// closed with 204, not an error status.
func TestHasJoinedHA5xxMojangDisabled(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	c := newCache()
	eng := buildRouter(t, srv, nil, c) // no mojang service

	w := doRequest(eng, http.MethodGet, hasJoinedURL("x"), "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", w.Code, w.Body.String())
	}
}

// TestHasJoinedHA5xxMojangMiss covers HA failure plus a negative Mojang
// answer: nothing verified the join, so the result is 204.
func TestHasJoinedHA5xxMojangMiss(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		return nil, hrpauth.ErrNoProfile
	}}
	c := newCache()
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("x"), "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", w.Code, w.Body.String())
	}
}

// TestHasJoinedRegister409OtherCode covers a 409 whose error body is NOT
// username_already_bound: the client maps it to ErrUpstream and the
// handler answers 503 (auth backend unavailable), not 204.
func TestHasJoinedRegister409OtherCode(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		return &hrpauth.PlayerProfile{ID: "moj-uuid-1", Name: "alice"}, nil
	}}
	ha.register = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "some_other_conflict"})
	}
	c := newCache()
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("alice"), "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body=%s)", w.Code, w.Body.String())
	}
}

// TestHasJoinedRegister400InvalidUUID covers the upstream rejecting the
// proxy-register body (400 invalid_mojang_uuid): the client maps it to
// ErrInvalidInput and the handler surfaces 503.
func TestHasJoinedRegister400InvalidUUID(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		return &hrpauth.PlayerProfile{ID: "bad-uuid", Name: "alice"}, nil
	}}
	ha.register = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_mojang_uuid"})
	}
	c := newCache()
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("alice"), "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body=%s)", w.Code, w.Body.String())
	}
}

// TestHasJoinedStage2CacheHitUsernameBound covers the cache
// short-circuit variant of the bound rejection: a cached Mojang profile
// skips the Mojang roundtrip, the register still 409s with
// username_already_bound, and the join is rejected with 204.
func TestHasJoinedStage2CacheHitUsernameBound(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	mj := &fakeMojang{hasJoinedFn: func(url.Values) (*hrpauth.PlayerProfile, error) {
		t.Fatal("mojang must not be called on stage-2 cache hit")
		return nil, nil
	}}
	ha.register = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "username_already_bound"})
	}
	c := newCache()
	c.SetMojangProfile("alice", &hrpauth.PlayerProfile{ID: "moj-uuid-1", Name: "alice"})
	eng := buildRouter(t, srv, mj, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("alice"), "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", w.Code, w.Body.String())
	}
}

// TestHasJoinedStage2CacheHitRegister5xx covers the cache short-circuit
// with a failing register: the handler answers 503.
func TestHasJoinedStage2CacheHitRegister5xx(t *testing.T) {
	ha, srv := newFakeHA(t)
	ha.hasJoin = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	ha.register = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	c := newCache()
	c.SetMojangProfile("alice", &hrpauth.PlayerProfile{ID: "moj-uuid-1", Name: "alice"})
	eng := buildRouter(t, srv, &fakeMojang{}, c)

	w := doRequest(eng, http.MethodGet, hasJoinedURL("alice"), "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body=%s)", w.Code, w.Body.String())
	}
}
