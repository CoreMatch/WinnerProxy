package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/winnerproxy/winnerproxy/config"
	"bytes"
)

var (
	ErrNoProfile = errors.New("no profile found")
	ErrMultiAuth = errors.New("multiple services returned profiles")
)

type PlayerProfile struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Properties []PlayerProperty    `json:"properties,omitempty"`
}

type PlayerProperty struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

type UpstreamService interface {
	HasJoined(params url.Values) (*PlayerProfile, error)
	QueryProfile(uuid string, unsigned bool) (*PlayerProfile, error)
	BatchQuery(names []string) ([]*PlayerProfile, error)
	ID() string
}

type MojangService struct {
	client *http.Client
}

func NewMojangService(timeout time.Duration) *MojangService {
	return &MojangService{
		client: &http.Client{Timeout: timeout},
	}
}

func (m *MojangService) ID() string { return "mojang" }

func (m *MojangService) HasJoined(params url.Values) (*PlayerProfile, error) {
	reqURL := "https://sessionserver.mojang.com/session/minecraft/hasJoined?" + params.Encode()
	resp, err := m.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoProfile
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mojang hasJoined failed: %d %s", resp.StatusCode, string(body))
	}

	var profile PlayerProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (m *MojangService) QueryProfile(uuid string, unsigned bool) (*PlayerProfile, error) {
	reqURL := fmt.Sprintf("https://sessionserver.mojang.com/session/minecraft/profile/%s?unsigned=%t", uuid, unsigned)
	resp, err := m.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoProfile
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mojang queryProfile failed: %d %s", resp.StatusCode, string(body))
	}

	var profile PlayerProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (m *MojangService) BatchQuery(names []string) ([]*PlayerProfile, error) {
	data, err := json.Marshal(names)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.minecraftservices.com/minecraft/profile/lookup/bulk/byname", nil)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mojang batchQuery failed: %d %s", resp.StatusCode, string(body))
	}

	var profiles []*PlayerProfile
	if err := json.NewDecoder(resp.Body).Decode(&profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

type YggdrasilService struct {
	client    *http.Client
	baseURL   string
	serviceID string
}

func NewYggdrasilService(baseURL, serviceID string, timeout time.Duration) *YggdrasilService {
	return &YggdrasilService{
		client:    &http.Client{Timeout: timeout},
		baseURL:   baseURL,
		serviceID: serviceID,
	}
}

func (y *YggdrasilService) ID() string { return y.serviceID }

func (y *YggdrasilService) HasJoined(params url.Values) (*PlayerProfile, error) {
	reqURL := y.baseURL + "/sessionserver/session/minecraft/hasJoined?" + params.Encode()
	resp, err := y.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoProfile
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("yggdrasil hasJoined failed: %d %s", resp.StatusCode, string(body))
	}

	var profile PlayerProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (y *YggdrasilService) QueryProfile(uuid string, unsigned bool) (*PlayerProfile, error) {
	reqURL := fmt.Sprintf("%s/sessionserver/session/minecraft/profile/%s?unsigned=%t", y.baseURL, uuid, unsigned)
	resp, err := y.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoProfile
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("yggdrasil queryProfile failed: %d %s", resp.StatusCode, string(body))
	}

	var profile PlayerProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (y *YggdrasilService) BatchQuery(names []string) ([]*PlayerProfile, error) {
	data, err := json.Marshal(names)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", y.baseURL+"/api/profiles/minecraft", nil)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := y.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("yggdrasil batchQuery failed: %d %s", resp.StatusCode, string(body))
	}

	var profiles []*PlayerProfile
	if err := json.NewDecoder(resp.Body).Decode(&profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

type Proxy struct {
	services []UpstreamService
	cfg      *config.Config
}

func New(cfg *config.Config) *Proxy {
	var services []UpstreamService

	if cfg.Upstreams.Official.Enabled {
		timeout := time.Duration(cfg.Upstreams.Official.TimeoutSec) * time.Second
		services = append(services, NewMojangService(timeout))
	}

	if cfg.Upstreams.YggdrasilAPI.Enabled && cfg.Upstreams.YggdrasilAPI.URL != "" {
		timeout := time.Duration(cfg.Upstreams.YggdrasilAPI.TimeoutSec) * time.Second
		services = append(services, NewYggdrasilService(cfg.Upstreams.YggdrasilAPI.URL, "yggdrasil", timeout))
	}

	return &Proxy{services: services, cfg: cfg}
}

func (p *Proxy) GetServices() []UpstreamService {
	return p.services
}
