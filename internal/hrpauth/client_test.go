package hrpauth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newTestClient returns a Client pointing at the given test server URL,
// with a fixed M.T. and the default *http.Client as the doer.
func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	return New(baseURL, "test-manage-token", nil)
}

func TestNew_TrimsTrailingSlash(t *testing.T) {
	c := New("http://example.com/", "tok", nil)
	if c.baseURL != "http://example.com" {
		t.Fatalf("expected trimmed baseURL, got %q", c.baseURL)
	}
}

func TestNew_NilDoerDefaultsToHTTPClient(t *testing.T) {
	c := New("http://x", "tok", nil)
	if c.http == nil {
		t.Fatal("expected non-nil http doer")
	}
}

// --- HasJoined ---

func TestHasJoined_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/yggdrasil/sessionserver/session/minecraft/hasJoined" {
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
	}))
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

func TestHasJoined_204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.HasJoined(url.Values{"username": {"bob"}})
	if err != ErrNoProfile {
		t.Fatalf("expected ErrNoProfile, got %v", err)
	}
}

func TestHasJoined_5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.HasJoined(url.Values{"username": {"bob"}})
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestHasJoined_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // close immediately so connect fails

	c := newTestClient(t, srv.URL)
	_, err := c.HasJoined(url.Values{"username": {"bob"}})
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestHasJoined_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "not-json")
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.HasJoined(url.Values{"username": {"bob"}})
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

// --- GetProfile ---

func TestGetProfile_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("unsigned") != "true" {
			t.Errorf("expected unsigned=true, got %q", r.URL.Query().Get("unsigned"))
		}
		_ = json.NewEncoder(w).Encode(PlayerProfile{ID: "uuid", Name: "alice"})
	}))
	defer srv.Close()

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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("unsigned") != "" {
			t.Errorf("did not expect unsigned param, got %q", r.URL.Query().Get("unsigned"))
		}
		_ = json.NewEncoder(w).Encode(PlayerProfile{ID: "uuid", Name: "alice"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.GetProfile("uuid", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetProfile_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.GetProfile("nope", false)
	if err != ErrNoProfile {
		t.Fatalf("expected ErrNoProfile, got %v", err)
	}
}

// --- BatchQuery ---

func TestBatchQuery_200(t *testing.T) {
	var gotReq []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		_ = json.NewEncoder(w).Encode([]*PlayerProfile{
			{ID: "id1", Name: "alice"},
			{ID: "id2", Name: "bob"},
		})
	}))
	defer srv.Close()

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

func TestBatchQuery_5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.BatchQuery([]string{"alice"})
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

// --- RegisterByProxy ---

func TestRegisterByProxy_200(t *testing.T) {
	var gotBody RegisterRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(RegisterResponse{
			Success:   true,
			UID:       42,
			Message:   "Register successful",
			ProfileID: "f7c77d999f154a66a87dc4a51ef30d19",
			CBH:       0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	got, err := c.RegisterByProxy("alice", "f7c77d999f154a66a87dc4a51ef30d19", "pw123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProfileID != "f7c77d999f154a66a87dc4a51ef30d19" || got.CBH != 0 || got.UID != 42 {
		t.Errorf("unexpected response: %+v", got)
	}
	if gotBody.RememberToken != "test-manage-token" {
		t.Errorf("M.T. not injected: %q", gotBody.RememberToken)
	}
	if gotBody.Username != "alice" || gotBody.MojangUUID != "f7c77d999f154a66a87dc4a51ef30d19" {
		t.Errorf("body fields wrong: %+v", gotBody)
	}
	if gotBody.Email != "" {
		t.Errorf("email should be empty (HA fills placeholder), got %q", gotBody.Email)
	}
}

func TestRegisterByProxy_409_UsernameBound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "username_already_bound"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.RegisterByProxy("alice", "uuid", "pw")
	if err != ErrUsernameBound {
		t.Fatalf("expected ErrUsernameBound, got %v", err)
	}
}

func TestRegisterByProxy_400_InvalidMojangUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_mojang_uuid"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.RegisterByProxy("alice", "not-hex", "pw")
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestRegisterByProxy_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.RegisterByProxy("alice", "uuid", "pw")
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestRegisterByProxy_409_OtherError(t *testing.T) {
	// 409 with unknown error code → ErrUpstream (defensive)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "weird_state"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.RegisterByProxy("alice", "uuid", "pw")
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestRegisterByProxy_400_OtherError(t *testing.T) {
	// 400 with unknown error code → ErrUpstream
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "weird_state"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.RegisterByProxy("alice", "uuid", "pw")
	if err != ErrUpstream {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestRegisterByProxy_ContentType(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		_ = json.NewEncoder(w).Encode(RegisterResponse{Success: true})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, _ = c.RegisterByProxy("alice", "uuid", "pw")
	if !strings.HasPrefix(gotCT, "application/json") {
		t.Errorf("expected Content-Type: application/json, got %q", gotCT)
	}
}
