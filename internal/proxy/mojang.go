package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
)

// MojangService talks to the official Mojang services. The wire
// PlayerProfile shape is identical to hrpauth.PlayerProfile, so we
// decode directly into the shared type.
type MojangService struct {
	baseURL string
	client  *http.Client
}

// NewMojangService returns a MojangService with the given per-request
// timeout and base URL. Pass 0 to disable the timeout.
func NewMojangService(baseURL string, timeout time.Duration) *MojangService {
	return &MojangService{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}
}

// ID implements UpstreamService.
func (m *MojangService) ID() string { return "mojang" }

// HasJoined implements UpstreamService.
func (m *MojangService) HasJoined(params url.Values) (*hrpauth.PlayerProfile, error) {
	u, err := url.JoinPath(m.baseURL, "/session/minecraft/hasJoined")
	if err != nil {
		return nil, hrpauth.ErrUpstream
	}
	resp, err := m.client.Get(u + "?" + params.Encode())
	if err != nil {
		return nil, hrpauth.ErrUpstream
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var p hrpauth.PlayerProfile
		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			return nil, hrpauth.ErrUpstream
		}
		return &p, nil
	case http.StatusNoContent, http.StatusNotFound:
		return nil, hrpauth.ErrNoProfile
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mojang hasJoined: %d %s", resp.StatusCode, string(body))
	}
}

// QueryProfile implements UpstreamService.
func (m *MojangService) QueryProfile(uuid string, unsigned bool) (*hrpauth.PlayerProfile, error) {
	u, err := url.JoinPath(m.baseURL, "/session/minecraft/profile/"+uuid)
	if err != nil {
		return nil, hrpauth.ErrUpstream
	}
	resp, err := m.client.Get(fmt.Sprintf("%s?unsigned=%t", u, unsigned))
	if err != nil {
		return nil, hrpauth.ErrUpstream
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var p hrpauth.PlayerProfile
		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			return nil, hrpauth.ErrUpstream
		}
		return &p, nil
	case http.StatusNoContent, http.StatusNotFound:
		return nil, hrpauth.ErrNoProfile
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mojang queryProfile: %d %s", resp.StatusCode, string(body))
	}
}

// BatchQuery implements UpstreamService. Returns an empty slice on
// 200-with-no-results so callers can iterate without nil-check.
func (m *MojangService) BatchQuery(names []string) ([]*hrpauth.PlayerProfile, error) {
	data, err := json.Marshal(names)
	if err != nil {
		return nil, err
	}
	u, err := url.JoinPath(m.baseURL, "/minecraft/profile/lookup/bulk/byname")
	if err != nil {
		return nil, hrpauth.ErrUpstream
	}
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, hrpauth.ErrUpstream
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var ps []*hrpauth.PlayerProfile
		if err := json.NewDecoder(resp.Body).Decode(&ps); err != nil {
			return nil, hrpauth.ErrUpstream
		}
		return ps, nil
	case http.StatusNoContent, http.StatusNotFound:
		return []*hrpauth.PlayerProfile{}, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mojang batchQuery: %d %s", resp.StatusCode, string(body))
	}
}
