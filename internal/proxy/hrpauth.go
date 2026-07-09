package proxy

import (
	"net/url"

	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
)

// HrpauthService adapts hrpauth.Client to the UpstreamService interface.
// It is used by handler for QueryProfile / BatchQuery only; HasJoined is
// handled directly in handler.go per the three-stage flow.
type HrpauthService struct {
	cli *hrpauth.Client
}

// NewHrpauthService wraps an existing hrpauth.Client.
func NewHrpauthService(cli *hrpauth.Client) *HrpauthService {
	return &HrpauthService{cli: cli}
}

// ID implements UpstreamService.
func (h *HrpauthService) ID() string { return "hrpauth" }

// HasJoined implements UpstreamService (passes through).
func (h *HrpauthService) HasJoined(params url.Values) (*hrpauth.PlayerProfile, error) {
	return h.cli.HasJoined(params)
}

// QueryProfile implements UpstreamService.
func (h *HrpauthService) QueryProfile(uuid string, unsigned bool) (*hrpauth.PlayerProfile, error) {
	return h.cli.GetProfile(uuid, unsigned)
}

// BatchQuery implements UpstreamService.
func (h *HrpauthService) BatchQuery(names []string) ([]*hrpauth.PlayerProfile, error) {
	return h.cli.BatchQuery(names)
}
