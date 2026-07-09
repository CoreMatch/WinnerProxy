package hrpauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
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
)

// Client is a thin HTTP client for HRPAuth. One instance per WinnerProxy
// process; safe for concurrent use because *http.Client is.
type Client struct {
	baseURL     string
	manageToken string
	http        HTTPDoer
}

// New constructs a Client. If doer is nil, a *http.Client with a 10s
// timeout is used. manageToken is sent on every RegisterByProxy call
// in the remember_token body field (see HA-ROADMAP §3.1).
func New(baseURL, manageToken string, doer HTTPDoer) *Client {
	if doer == nil {
		doer = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		manageToken: manageToken,
		http:        doer,
	}
}

// doGet issues a GET against path?query. The caller decides what each
// status code means; this helper only builds the request and executes it.
func (c *Client) doGet(path, query string) (*http.Response, error) {
	u := c.baseURL + path
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
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return c.http.Do(req)
}
