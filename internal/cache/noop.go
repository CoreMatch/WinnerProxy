package cache

import "github.com/winnerproxy/winnerproxy/internal/hrpauth"

// Noop is a ProfileCache that never stores anything. It is used when
// cache.size=0 in the config. All Get* methods return (nil, false);
// all Set* methods are no-ops.
type Noop struct{}

// NewNoop returns an empty ProfileCache. Always non-nil.
func NewNoop() *Noop { return &Noop{} }

// GetHAProfile implements ProfileCache.
func (Noop) GetHAProfile(string) (*hrpauth.PlayerProfile, bool) { return nil, false }

// SetHAProfile implements ProfileCache.
func (Noop) SetHAProfile(string, *hrpauth.PlayerProfile) error { return nil }

// GetMojangProfile implements ProfileCache.
func (Noop) GetMojangProfile(string) (*hrpauth.PlayerProfile, bool) { return nil, false }

// SetMojangProfile implements ProfileCache.
func (Noop) SetMojangProfile(string, *hrpauth.PlayerProfile) error { return nil }
