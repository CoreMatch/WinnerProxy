package hrpauth

import (
	"encoding/json"
	"io"
	"net/url"
)

// HasJoined forwards the Yggdrasil hasJoined request to HA. The query
// is passed through verbatim so any future HA-side parameters (e.g. ip)
// are supported without WinnerProxy changes.
//
//	200 + body   → *PlayerProfile, nil
//	204          → nil, ErrNoProfile
//	network err  → nil, ErrUpstream
//	other status → nil, ErrUpstream
func (c *Client) HasJoined(q url.Values) (*PlayerProfile, error) {
	resp, err := c.doGet("/yggdrasil/sessionserver/session/minecraft/hasJoined", q.Encode())
	if err != nil {
		return nil, ErrUpstream
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		var p PlayerProfile
		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			return nil, ErrUpstream
		}
		return &p, nil
	case 204:
		return nil, ErrNoProfile
	default:
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, ErrUpstream
	}
}

// GetProfile fetches a profile by UUID from HA. The unsigned flag maps
// to the Yggdrasil `unsigned=true` query parameter.
//
//	200 + body   → *PlayerProfile, nil
//	204 / 404    → nil, ErrNoProfile
//	network err  → nil, ErrUpstream
//	other status → nil, ErrUpstream
func (c *Client) GetProfile(uuid string, unsigned bool) (*PlayerProfile, error) {
	q := url.Values{}
	if unsigned {
		q.Set("unsigned", "true")
	}
	resp, err := c.doGet("/yggdrasil/sessionserver/session/minecraft/profile/"+uuid, q.Encode())
	if err != nil {
		return nil, ErrUpstream
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		var p PlayerProfile
		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			return nil, ErrUpstream
		}
		return &p, nil
	case 204, 404:
		return nil, ErrNoProfile
	default:
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, ErrUpstream
	}
}

// BatchQuery forwards a list of usernames to HA's Yggdrasil batch API.
// Per Yggdrasil convention, only the {id, name} summary is returned; the
// textures properties are deliberately dropped.
//
//	200 + body   → []*PlayerProfile, nil (may be empty)
//	network err  → nil, ErrUpstream
//	other status → nil, ErrUpstream
func (c *Client) BatchQuery(names []string) ([]*PlayerProfile, error) {
	resp, err := c.doPost("/yggdrasil/api/profiles/minecraft", names)
	if err != nil {
		return nil, ErrUpstream
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, ErrUpstream
	}
	var out []*PlayerProfile
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, ErrUpstream
	}
	return out, nil
}

// RegisterByProxy performs an M.T.-authenticated POST /register against
// HA. The M.T. is sent in the remember_token body field (HA-ROADMAP
// §3.1) and is not exposed in HTTP headers.
//
//	200                → *RegisterResponse, nil
//	409 + err=username_already_bound → nil, ErrUsernameBound
//	400 + err=invalid_mojang_uuid    → nil, ErrInvalidInput
//	5xx / network / decode error     → nil, ErrUpstream
//	other 4xx                        → nil, ErrUpstream
func (c *Client) RegisterByProxy(username, mojangUUID, password string) (*RegisterResponse, error) {
	body := RegisterRequest{
		Username:      username,
		Password:      password,
		Email:         "", // HA auto-fills placeholder in M.T. path
		MojangUUID:    mojangUUID,
		RememberToken: c.manageToken,
	}
	resp, err := c.doPost("/register", body)
	if err != nil {
		return nil, ErrUpstream
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		var rr RegisterResponse
		if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
			return nil, ErrUpstream
		}
		return &rr, nil
	case 409:
		eb := decodeErrorBody(resp.Body)
		if eb == "username_already_bound" {
			return nil, ErrUsernameBound
		}
		return nil, ErrUpstream
	case 400:
		eb := decodeErrorBody(resp.Body)
		if eb == "invalid_mojang_uuid" {
			return nil, ErrInvalidInput
		}
		return nil, ErrUpstream
	default:
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, ErrUpstream
	}
}

// decodeErrorBody reads a {"error": "..."} body and returns the error
// code. Decode failures are silently treated as "".
func decodeErrorBody(r io.Reader) string {
	var eb struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(r).Decode(&eb)
	return eb.Error
}

// GetServerMeta fetches HA's Yggdrasil root metadata (skinDomains,
// signaturePublickey, etc). WinnerProxy exposes this at "/" and
// "/yggdrasil" so legacy Mojang clients that probe the root still
// receive a valid Yggdrasil response.
//
//	200 + body   → map (raw JSON), nil
//	network err  → nil, ErrUpstream
//	other status → nil, ErrUpstream
func (c *Client) GetServerMeta() (map[string]any, error) {
	resp, err := c.doGet("/", "")
	if err != nil {
		return nil, ErrUpstream
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, ErrUpstream
	}
	var meta map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, ErrUpstream
	}
	return meta, nil
}
