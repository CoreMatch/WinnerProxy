// Package cache provides an in-process profile cache used by the
// handler to short-circuit repeated upstream lookups.
//
// Two keys are supported:
//   - HA profile (key: <uuid>), written whenever hrpauth returns a
//     profile (QueryProfile or HasJoined stage 1).
//   - Mojang profile (key: <username>), written when the Mojang
//     fallback in HasJoined stage 2 succeeds.
//
// Negative responses (HRPAuth 204, Mojang 204, upstream errors) are
// deliberately NOT cached so the proxy still has a chance to pick
// up newly-created accounts.
package cache

import "github.com/winnerproxy/winnerproxy/internal/hrpauth"

// ProfileCache is the interface the handler uses. The implementation
// is concurrency-safe.
type ProfileCache interface {
	// GetHAProfile returns the cached HRPAuth profile for uuid, or
	// (nil, false) on miss.
	GetHAProfile(uuid string) (*hrpauth.PlayerProfile, bool)
	// SetHAProfile stores p under uuid for the configured TTL.
	// Errors are non-fatal: callers should log and continue.
	SetHAProfile(uuid string, p *hrpauth.PlayerProfile) error

	// GetMojangProfile returns the cached Mojang profile for username,
	// or (nil, false) on miss.
	GetMojangProfile(username string) (*hrpauth.PlayerProfile, bool)
	// SetMojangProfile stores p under username for the configured TTL.
	SetMojangProfile(username string, p *hrpauth.PlayerProfile) error
}
