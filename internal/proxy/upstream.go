// Package proxy is a thin abstraction over Yggdrasil upstream services.
// It contains the UpstreamService interface and concrete implementations
// for the official Mojang sessionserver and HRPAuth.
package proxy

import (
	"net/url"

	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
)

// UpstreamService is the minimal interface a Yggdrasil upstream must
// implement. The handler uses it for QueryProfile / BatchQuery; HasJoined
// is handled directly via the three-stage flow in P3.
type UpstreamService interface {
	HasJoined(params url.Values) (*hrpauth.PlayerProfile, error)
	QueryProfile(uuid string, unsigned bool) (*hrpauth.PlayerProfile, error)
	BatchQuery(names []string) ([]*hrpauth.PlayerProfile, error)
	ID() string
}
