package hrpauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// HTTPDoer is the minimal interface the client needs from net/http.
// Defining it as an interface lets tests inject a *http.Client backed
// by httptest.NewServer.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Sentinel errors returned by the client. The handler switches on
// these via errors.Is.
var (
	// ErrNoProfile means HA returned 204 (or an empty profile) — the
	// player has no active session.
	ErrNoProfile = errors.New("hrpauth: no profile")

	// ErrUsernameBound means HA returned 409 with error code
	// "username_already_bound" — the username is taken by a non-bindable
	// HA account; the Mojang player must be rejected.
	ErrUsernameBound = errors.New("hrpauth: username already bound")

	// ErrInvalidInput means HA returned 400 with a stable error code
	// (e.g. "invalid_mojang_uuid") — the request was malformed.
	ErrInvalidInput = errors.New("hrpauth: invalid input")

	// ErrUpstream covers 5xx, network errors, timeouts, and JSON
	// decode failures. The handler treats this as transient.
	ErrUpstream = errors.New("hrpauth: upstream unavailable")

	// ErrEndpointMissing means HA answered 404 for a route that is
	// supposed to exist — typically relay-prefix or contract drift.
	// The handler maps it to an explicit 502, never a bare 500.
	ErrEndpointMissing = errors.New("hrpauth: endpoint missing (upstream 404)")

	// ErrSDKPackageConflict means the SDK package already exists (409).
	ErrSDKPackageConflict = errors.New("hrpauth: sdk package conflict")
)

// Client is a thin HTTP client for HRPAuth. One instance per WinnerProxy
// process; safe for concurrent use.
type Client struct {
	baseURL string
	rootURL string
	http    HTTPDoer

	mu          sync.Mutex
	serviceName string // presence-registry name, used when fetching the internal key
	internalKey string // X-Internal-Key value, empty until fetched
}

// New constructs a Client. It initializes an OAuth2 client_credentials
// flow using the provided clientID and clientSecret. The returned client
// automatically attaches a Bearer service token to every request.
func New(baseURL, clientID, clientSecret string, doer HTTPDoer) *Client {
	ctx := context.Background()
	if doer != nil {
		// If a custom doer is provided (e.g. in tests), we must ensure
		// oauth2 uses it for token exchange.
		if httpClient, ok := doer.(*http.Client); ok {
			ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
		}
	}

	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)

	// Standard HRPAuth OAuth token endpoint is /oauth/token; the internal
	// key lives on the core's /services/auth-key. Both sit on the host root
	// of the configured base URL, even when the base carries the
	// /yggdrasil-api relay prefix.
	tokenURL := "/oauth/token"
	rootURL := strings.TrimRight(baseURL, "/")
	if u, err := url.Parse(baseURL); err == nil {
		tokenURL = fmt.Sprintf("%s://%s/oauth/token", u.Scheme, u.Host)
		rootURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}

	conf := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		rootURL: rootURL,
		http:    conf.Client(ctx),
	}
}

// SetServiceName records this service's presence-registry name, used when
// fetching the X-Internal-Key for proxy registration.
func (c *Client) SetServiceName(name string) {
	c.mu.Lock()
	c.serviceName = name
	c.mu.Unlock()
}

// FetchInternalKey obtains (or refreshes) this service's X-Internal-Key from
// the HRPAuth core endpoint POST /services/auth-key, authenticated by the
// client's service token. The key is stored and attached to proxy-registration
// requests. The core re-issues the key on every call, so this is also the
// recovery path after a 401.
func (c *Client) FetchInternalKey() error {
	c.mu.Lock()
	name := c.serviceName
	c.mu.Unlock()
	if name == "" {
		return errors.New("hrpauth: service name not configured (SetServiceName)")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(map[string]string{"name": name}); err != nil {
		return err
	}
	u, err := url.JoinPath(c.rootURL, "/services/auth-key")
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, u, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("hrpauth: fetch internal key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("hrpauth: fetch internal key: upstream status %d", resp.StatusCode)
	}
	var result struct {
		InternalKey string `json:"internal_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("hrpauth: fetch internal key: %w", err)
	}
	if result.InternalKey == "" {
		return errors.New("hrpauth: fetch internal key: empty key in response")
	}

	c.mu.Lock()
	c.internalKey = result.InternalKey
	c.mu.Unlock()
	return nil
}

func (c *Client) internalKeyState() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.internalKey
}

// doGet issues a GET against path?query. The caller decides what each
// status code means; this helper only builds the request and executes it.
func (c *Client) doGet(path, query string) (*http.Response, error) {
	u, err := url.JoinPath(c.baseURL, path)
	if err != nil {
		return nil, err
	}
	if query != "" {
		u += "?" + query
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return c.http.Do(req)
}

// doPost issues a POST with a JSON-encoded body. The caller decides
// what each status code means.
func (c *Client) doPost(path string, body interface{}) (*http.Response, error) {
	return c.doPostHeaders(path, body, nil)
}

// doPostHeaders issues a POST with a JSON-encoded body plus extra request
// headers. Used for the internal-key-authenticated proxy registration.
func (c *Client) doPostHeaders(path string, body interface{}, headers map[string]string) (*http.Response, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}
	u, err := url.JoinPath(c.baseURL, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, u, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return c.http.Do(req)
}
