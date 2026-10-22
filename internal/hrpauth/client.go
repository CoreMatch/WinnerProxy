package hrpauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

	// ErrSDKPackageConflict means the SDK package already exists (409).
	ErrSDKPackageConflict = errors.New("hrpauth: sdk package conflict")
)

// Client is a thin HTTP client for HRPAuth. One instance per WinnerProxy
// process; safe for concurrent use because *http.Client is.
type Client struct {
	baseURL string
	http    HTTPDoer
}

// New constructs a Client. It initializes an OAuth2 client_credentials
// flow using the provided clientID and clientSecret. The returned client
// automatically attaches a Bearer token to every request.
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

	// Standard HRPAuth OAuth token endpoint is /oauth/token.
	// We always use the root of the provided baseURL for the token endpoint
	// to ensure it works even if a subpath (like /yggdrasil-api/) is provided.
	tokenURL := "/oauth/token"
	if u, err := url.Parse(baseURL); err == nil {
		tokenURL = fmt.Sprintf("%s://%s/oauth/token", u.Scheme, u.Host)
	}

	conf := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    conf.Client(ctx),
	}
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
	return c.http.Do(req)
}
