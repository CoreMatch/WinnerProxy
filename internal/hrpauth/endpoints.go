package hrpauth

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
)

// HasJoined forwards the Yggdrasil hasJoined request to HA. The query
// is passed through verbatim so any future HA-side parameters (e.g. ip)
// are supported without WinnerProxy changes.
//
// HA's router does NOT use a /yggdrasil prefix — it serves the
// Yggdrasil API at the prefix-less path (see
// HRPAuth-Backend-Go/main.go:106-127).
//
//	200 + body   → *PlayerProfile, nil
//	200 + {}     → nil, ErrNoProfile   (HA's "not found" shape)
//	204          → nil, ErrNoProfile
//	network err  → nil, ErrUpstream
//	other status → nil, ErrUpstream
func (c *Client) HasJoined(q url.Values) (*PlayerProfile, error) {
	resp, err := c.doGet("/sessionserver/session/minecraft/hasJoined", q.Encode())
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
		// HA returns 200 + {} when the profile or matching session is
		// not found (see HRPAuth controllers/yggdrasil_controller.go
		// HasJoined). The zero-value PlayerProfile would otherwise leak
		// to the game server as a "successful" but empty identity.
		if p.ID == "" {
			return nil, ErrNoProfile
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
	resp, err := c.doGet("/sessionserver/session/minecraft/profile/"+uuid, q.Encode())
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

// RegisterByProxy performs an OAuth2-authenticated POST /register against HA.
func (c *Client) RegisterByProxy(username, mojangUUID, password string) (*RegisterResponse, error) {
	body := RegisterRequest{
		Username:   username,
		Password:   password,
		Email:      "", // HA auto-fills placeholder in OAuth2 path
		MojangUUID: mojangUUID,
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

// RegisterPresence performs the microservice presence (bonjour)
// handshake: it registers or heartbeats WinnerProxy in HRPAuth's
// presence registry. It is fire-and-forget from the caller's point of
// view; failures surface as ErrUpstream and never stop the process.
//
//	200          → nil
//	network err  → ErrUpstream
//	other status → ErrUpstream
func (c *Client) RegisterPresence(req PresenceRequest) error {
	log.Printf("hrpauth: registering presence (name=%s, security_level=%d)", req.Name, req.SecurityLevel)
	resp, err := c.doPost("/services/presence", req)
	if err != nil {
		log.Printf("hrpauth: presence registration request failed: %v", err)
		return ErrUpstream
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("hrpauth: presence registration returned status %d", resp.StatusCode)
		_, _ = io.Copy(io.Discard, resp.Body)
		return ErrUpstream
	}
	log.Printf("hrpauth: presence registration successful")
	return nil
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

// DeclareEmail updates a user's email using OAuth2 authentication.
func (c *Client) DeclareEmail(email, playername string) error {
	body := DeclareEmailRequest{
		Email:      email,
		PlayerName: playername,
	}
	resp, err := c.doPost("/user/declare-email", body)
	if err != nil {
		return ErrUpstream
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ErrUpstream
	}
	return nil
}

// EnableMojangBind enables the mbe (Mojang Bind Enabled) flag for a user
// using OAuth2 authentication.
func (c *Client) EnableMojangBind(uid int64) error {
	body := EnableMojangBindRequest{
		UID: strconv.FormatInt(uid, 10),
	}
	resp, err := c.doPost("/user/mojang-bind-enable", body)
	if err != nil {
		return ErrUpstream
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ErrUpstream
	}
	return nil
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

// RegisterRelay registers relay rules for the microservice.
func (c *Client) RegisterRelay(name string, rules []RelayRule) error {
	log.Printf("hrpauth: registering %d relay rules for %s", len(rules), name)
	req := RegisterRelayRequest{
		Name:   name,
		Relays: rules,
	}
	resp, err := c.doPost("/services/relay", req)
	if err != nil {
		log.Printf("hrpauth: relay registration request failed: %v", err)
		return ErrUpstream
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("hrpauth: relay registration returned status %d", resp.StatusCode)
		return ErrUpstream
	}
	log.Printf("hrpauth: relay registration successful")
	return nil
}

// UploadSDKPackage uploads an SDK package archive to HRPAuth.
func (c *Client) UploadSDKPackage(archive []byte, filename string) error {
	log.Printf("hrpauth: uploading sdk package %s (%d bytes)", filename, len(archive))
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("package", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(archive); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}

	u, err := url.JoinPath(c.baseURL, "/services/sdk-packages")
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, u, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		log.Printf("hrpauth: sdk upload request failed: %v", err)
		return ErrUpstream
	}
	defer resp.Body.Close()

	log.Printf("hrpauth: sdk upload returned status %d", resp.StatusCode)
	switch resp.StatusCode {
	case 200, 201:
		return nil
	case 409:
		return ErrSDKPackageConflict
	default:
		return ErrUpstream
	}
}

// DeleteSDKPackage deletes an SDK package from HRPAuth.
func (c *Client) DeleteSDKPackage(name string) error {
	log.Printf("hrpauth: deleting sdk package %s", name)
	u, err := url.JoinPath(c.baseURL, "/services/sdk-packages/"+name)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		log.Printf("hrpauth: sdk delete request failed: %v", err)
		return ErrUpstream
	}
	defer resp.Body.Close()

	log.Printf("hrpauth: sdk delete returned status %d", resp.StatusCode)
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return ErrUpstream
	}
	return nil
}
