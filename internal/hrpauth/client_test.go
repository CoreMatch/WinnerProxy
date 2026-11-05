package hrpauth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// newTestClient returns a Client pointing at the given test server URL,
// with fixed credentials and the default *http.Client as the doer.
func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	return New(baseURL, "test-client-id", "test-client-secret", nil)
}

func TestNew_TrimsTrailingSlash(t *testing.T) {
	c := New("http://example.com/", "id", "secret", nil)
	if c.baseURL != "http://example.com" {
		t.Fatalf("expected trimmed baseURL, got %q", c.baseURL)
	}
}

func TestNew_NilDoerDefaultsToHTTPClient(t *testing.T) {
	c := New("http://x", "id", "secret", nil)
	if c.http == nil {
		t.Fatal("expected non-nil http doer")
	}
}

// --- HasJoined ---

func TestHasJoined_200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/sessionserver/session/minecraft/hasJoined", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("missing or invalid Authorization header: %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/sessionserver/session/minecraft/hasJoined" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("username"); got != "alice" {
			t.Errorf("username mismatch: %s", got)
		}
		if got := r.URL.Query().Get("serverId"); got != "abc" {
			t.Errorf("serverId mismatch: %s", got)
		}
		if got := r.URL.Query().Get("ip"); got != "1.2.3.4" {
			t.Errorf("ip passthrough failed: %s", got)
		}
		_ = json.NewEncoder(w).Encode(PlayerProfile{
			ID:   "f7c77d999f154a66a87dc4a51ef30d19",
			Name: "alice",
			Properties: []PlayerProperty{
				{Name: "textures", Value: "v", Signature: "s"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	got, err := c.HasJoined(url.Values{
		"username": {"alice"},
		"serverId": {"abc"},
		"ip":       {"1.2.3.4"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "f7c77d999f154a66a87dc4a51ef30d19" || got.Name != "alice" {
		t.Errorf("unexpected profile: %+v", got)
	}
	if len(got.Properties) != 1 || got.Properties[0].Name != "textures" {
		t.Errorf("properties mismatch: %+v", got.Properties)
	}
}

// newTestServer returns a mux and a server that already handles /oauth/token.
func newTestServer(t *testing.T) (*http.ServeMux, *httptest.Server) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return mux, srv
}

func TestHasJoined_204(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/sessionserver/session/minecraft/hasJoined", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	c := newTestClient(t, srv.URL)
	_, err := c.HasJoined(url.Values{"username": {"bob"}})
	if err != ErrNoProfile {
		t.Fatalf("expected ErrNoProfile, got %v", err)
	}
}

// TestHasJoined_200_EmptyBody covers HA's "not found" shape: 200 OK
// with an empty JSON object {}. The client must surface this as
// ErrNoProfile (matching the 204 path) rather than returning a
// zero-value profile to the game server.
func TestHasJoined_200_EmptyBody(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/sessionserver/session/minecraft/hasJoined", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{}")
	})

	c := newTestClient(t, srv.URL)
	got, err := c.HasJoined(url.Values{"username": {"bob"}})
	if err != ErrNoProfile {
		t.Fatalf("expected ErrNoProfile, got err=%v profile=%+v", err, got)
	}
	if got != nil {
		t.Fatalf("expected nil profile, got %+v", got)
	}
}

func TestHasJoined_5xx(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/sessionserver/session/minecraft/hasJoined", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})

	c := newTestClient(t, srv.URL)
	_, err := c.HasJoined(url.Values{"username": {"bob"}})
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestHasJoined_NetworkError(t *testing.T) {
	_, srv := newTestServer(t)
	srv.Close() // close immediately so connect fails

	c := newTestClient(t, srv.URL)
	_, err := c.HasJoined(url.Values{"username": {"bob"}})
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestHasJoined_BadJSON(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/sessionserver/session/minecraft/hasJoined", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "not-json")
	})

	c := newTestClient(t, srv.URL)
	_, err := c.HasJoined(url.Values{"username": {"bob"}})
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

// --- GetProfile ---

func TestGetProfile_200(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/sessionserver/session/minecraft/profile/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("unsigned") != "true" {
			t.Errorf("expected unsigned=true, got %q", r.URL.Query().Get("unsigned"))
		}
		_ = json.NewEncoder(w).Encode(PlayerProfile{ID: "uuid", Name: "alice"})
	})

	c := newTestClient(t, srv.URL)
	got, err := c.GetProfile("uuid", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "alice" {
		t.Errorf("unexpected name: %s", got.Name)
	}
}

func TestGetProfile_NoUnsignedParam(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/sessionserver/session/minecraft/profile/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("unsigned") != "" {
			t.Errorf("did not expect unsigned param, got %q", r.URL.Query().Get("unsigned"))
		}
		_ = json.NewEncoder(w).Encode(PlayerProfile{ID: "uuid", Name: "alice"})
	})

	c := newTestClient(t, srv.URL)
	_, err := c.GetProfile("uuid", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetProfile_404(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/sessionserver/session/minecraft/profile/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	c := newTestClient(t, srv.URL)
	_, err := c.GetProfile("nope", false)
	if err != ErrNoProfile {
		t.Fatalf("expected ErrNoProfile, got %v", err)
	}
}

// --- BatchQuery ---

func TestBatchQuery_200(t *testing.T) {
	var gotReq []string
	mux, srv := newTestServer(t)
	mux.HandleFunc("/api/profiles/minecraft", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		_ = json.NewEncoder(w).Encode([]*PlayerProfile{
			{ID: "id1", Name: "alice"},
			{ID: "id2", Name: "bob"},
		})
	})

	c := newTestClient(t, srv.URL)
	got, err := c.BatchQuery([]string{"alice", "bob"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].Name != "alice" || got[1].Name != "bob" {
		t.Errorf("unexpected result: %+v", got)
	}
	if len(gotReq) != 2 || gotReq[0] != "alice" {
		t.Errorf("unexpected request body: %+v", gotReq)
	}
}

func TestBatchQuery_404(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/api/profiles/minecraft", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	c := newTestClient(t, srv.URL)
	_, err := c.BatchQuery([]string{"alice"})
	if err != ErrEndpointMissing {
		t.Fatalf("expected ErrEndpointMissing, got %v", err)
	}
}

func TestBatchQuery_5xx(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/api/profiles/minecraft", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})

	c := newTestClient(t, srv.URL)
	_, err := c.BatchQuery([]string{"alice"})
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

// --- RegisterPresence ---

func TestRegisterPresence_200(t *testing.T) {
	var gotBody PresenceRequest
	mux, srv := newTestServer(t)
	mux.HandleFunc("/services/presence", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected Content-Type: application/json, got %q", ct)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"message":"ca va très bien, merci"}`)
	})

	c := newTestClient(t, srv.URL)
	if err := c.RegisterPresence(PresenceRequest{Name: "WinnerProxy", TTLSeconds: 0}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody.Name != "WinnerProxy" {
		t.Errorf("name mismatch: %q", gotBody.Name)
	}
	if gotBody.TTLSeconds != 0 {
		t.Errorf("ttl_seconds mismatch: %d", gotBody.TTLSeconds)
	}
}

func TestRegisterPresence_Non200(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/services/presence", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = io.WriteString(w, `{"success":false,"code":"invalid_request"}`)
	})

	c := newTestClient(t, srv.URL)
	if err := c.RegisterPresence(PresenceRequest{Name: "WinnerProxy"}); err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestRegisterPresence_NetworkError(t *testing.T) {
	_, srv := newTestServer(t)
	srv.Close() // close immediately so connect fails

	c := newTestClient(t, srv.URL)
	if err := c.RegisterPresence(PresenceRequest{Name: "WinnerProxy"}); err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

// --- FetchInternalKey ---

func TestFetchInternalKey_200(t *testing.T) {
	var gotBody map[string]string
	mux, srv := newTestServer(t)
	mux.HandleFunc("/services/auth-key", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected service Bearer token, got %q", r.Header.Get("Authorization"))
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"message":"Internal key generated/updated","data":{"internal_key":"key-1"},"internal_key":"key-1"}`)
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	if err := c.FetchInternalKey(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["name"] != "WinnerProxy" {
		t.Errorf("expected name=WinnerProxy, got %q", gotBody["name"])
	}
	if key := c.internalKeyState(); key != "key-1" {
		t.Errorf("stored key = %q, want key-1", key)
	}
}

func TestFetchInternalKey_Non200(t *testing.T) {
	mux, srv := newTestServer(t)
	mux.HandleFunc("/services/auth-key", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"success":false,"code":"invalid_request","error":"invalid_request","message":"service not found"}`)
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	if err := c.FetchInternalKey(); err == nil {
		t.Fatal("expected error for non-200 auth-key response")
	}
	if key := c.internalKeyState(); key != "" {
		t.Fatalf("key must stay empty after a failed fetch, got %q", key)
	}
}

func TestFetchInternalKey_NoServiceName(t *testing.T) {
	_, srv := newTestServer(t)
	c := newTestClient(t, srv.URL)
	if err := c.FetchInternalKey(); err == nil {
		t.Fatal("expected error when the service name is not configured")
	}
}

// --- RegisterByProxy ---

// registerTestServer wires the fake upstream for proxy registration:
// /oauth/token (via newTestServer), /services/auth-key issuing keys from
// the provided sequence, and /internal/proxy-register answering through
// the register handler while recording every received key.
func registerTestServer(t *testing.T, keys []string, register func(w http.ResponseWriter, r *http.Request, key string)) (*httptest.Server, *func() []string) {
	t.Helper()
	var mu = &struct {
		sync.Mutex
		issued   int
		received []string
	}{}
	mux, srv := newTestServer(t)
	mux.HandleFunc("/services/auth-key", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		idx := mu.issued
		mu.issued++
		mu.Unlock()
		key := keys[len(keys)-1]
		if idx < len(keys) {
			key = keys[idx]
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"internal_key":"`+key+`"}`)
	})
	mux.HandleFunc("/internal/proxy-register", func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Internal-Key")
		mu.Lock()
		mu.received = append(mu.received, key)
		mu.Unlock()
		register(w, r, key)
	})
	keysSeen := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, len(mu.received))
		copy(out, mu.received)
		return out
	}
	return srv, &keysSeen
}

func TestRegisterByProxy_FetchesKeyAndSendsHeader(t *testing.T) {
	var gotBody RegisterRequest
	srv, keysSeen := registerTestServer(t, []string{"key-1"}, func(w http.ResponseWriter, r *http.Request, key string) {
		if key != "key-1" {
			t.Errorf("X-Internal-Key = %q, want key-1", key)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(RegisterResponse{
			Success:   true,
			Message:   "Proxy account registered",
			ProfileID: "f7c77d999f154a66a87dc4a51ef30d19",
		})
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	got, err := c.RegisterByProxy("alice", "f7c77d999f154a66a87dc4a51ef30d19", "pw123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProfileID != "f7c77d999f154a66a87dc4a51ef30d19" {
		t.Errorf("unexpected response: %+v", got)
	}
	if gotBody.Username != "alice" || gotBody.MojangUUID != "f7c77d999f154a66a87dc4a51ef30d19" {
		t.Errorf("body fields wrong: %+v", gotBody)
	}
	if len((*keysSeen)()) != 1 {
		t.Fatalf("register calls = %d, want 1", len((*keysSeen)()))
	}
}

// TestRegisterByProxy_401RefetchRetry covers key rotation: the first
// attempt is rejected with 401, the client re-fetches the key and the
// retry succeeds.
func TestRegisterByProxy_401RefetchRetry(t *testing.T) {
	var attempts int
	srv, keysSeen := registerTestServer(t, []string{"stale-key", "fresh-key"}, func(w http.ResponseWriter, r *http.Request, key string) {
		attempts++
		if key != "fresh-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"success":false,"code":"invalid_credentials","error":"invalid_credentials","message":"invalid internal key"}`)
			return
		}
		_ = json.NewEncoder(w).Encode(RegisterResponse{Success: true, ProfileID: "pid-2"})
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	got, err := c.RegisterByProxy("alice", "uuid", "pw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProfileID != "pid-2" {
		t.Errorf("unexpected response: %+v", got)
	}
	if attempts != 2 {
		t.Fatalf("register attempts = %d, want 2 (401 then retry)", attempts)
	}
	seen := (*keysSeen)()
	if len(seen) != 2 || seen[0] != "stale-key" || seen[1] != "fresh-key" {
		t.Fatalf("keys seen by upstream = %v, want [stale-key fresh-key]", seen)
	}
}

// TestRegisterByProxy_KeyUnavailableSendsNothing: when no internal key can
// be fetched, the registration must fail without sending a keyless request.
func TestRegisterByProxy_KeyUnavailableSendsNothing(t *testing.T) {
	var registerHits int
	mux, srv := newTestServer(t)
	mux.HandleFunc("/services/auth-key", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/internal/proxy-register", func(w http.ResponseWriter, r *http.Request) {
		registerHits++
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	if _, err := c.RegisterByProxy("alice", "uuid", "pw"); err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
	if registerHits != 0 {
		t.Fatalf("register endpoint hit %d times, want 0 (no keyless request)", registerHits)
	}
}

func TestRegisterByProxy_409_UsernameBound(t *testing.T) {
	srv, _ := registerTestServer(t, []string{"key-1"}, func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(409)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "username_already_bound"})
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	_, err := c.RegisterByProxy("alice", "uuid", "pw")
	if err != ErrUsernameBound {
		t.Fatalf("expected ErrUsernameBound, got %v", err)
	}
}

func TestRegisterByProxy_400_InvalidMojangUUID(t *testing.T) {
	srv, _ := registerTestServer(t, []string{"key-1"}, func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_mojang_uuid"})
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	_, err := c.RegisterByProxy("alice", "not-hex", "pw")
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestRegisterByProxy_500(t *testing.T) {
	srv, _ := registerTestServer(t, []string{"key-1"}, func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(500)
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	_, err := c.RegisterByProxy("alice", "uuid", "pw")
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestRegisterByProxy_409_OtherError(t *testing.T) {
	// 409 with unknown error code → ErrUpstream (defensive)
	srv, _ := registerTestServer(t, []string{"key-1"}, func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(409)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "weird_state"})
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	_, err := c.RegisterByProxy("alice", "uuid", "pw")
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestRegisterByProxy_400_OtherError(t *testing.T) {
	// 400 with unknown error code → ErrUpstream
	srv, _ := registerTestServer(t, []string{"key-1"}, func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "weird_state"})
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	_, err := c.RegisterByProxy("alice", "uuid", "pw")
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestRegisterByProxy_ContentType(t *testing.T) {
	var gotCT string
	srv, _ := registerTestServer(t, []string{"key-1"}, func(w http.ResponseWriter, r *http.Request, _ string) {
		gotCT = r.Header.Get("Content-Type")
		_ = json.NewEncoder(w).Encode(RegisterResponse{Success: true})
	})

	c := newTestClient(t, srv.URL)
	c.SetServiceName("WinnerProxy")
	_, _ = c.RegisterByProxy("alice", "uuid", "pw")
	if !strings.HasPrefix(gotCT, "application/json") {
		t.Errorf("expected Content-Type: application/json, got %q", gotCT)
	}
}
