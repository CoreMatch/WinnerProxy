package cache_test

import (
	"testing"

	"github.com/winnerproxy/winnerproxy/internal/cache"
	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
)

func newCache() *cache.FreeCache {
	return cache.NewFreeCache(1<<20, 60_000_000_000) // 1 MiB, 60s TTL
}

func sampleProfile(id, name string) *hrpauth.PlayerProfile {
	return &hrpauth.PlayerProfile{
		ID:   id,
		Name: name,
		Properties: []hrpauth.PlayerProperty{{
			Name: "textures", Value: "v", Signature: "s",
		}},
	}
}

func TestHAProfile_RoundTrip(t *testing.T) {
	c := newCache()
	if err := c.SetHAProfile("uuid-1", sampleProfile("uuid-1", "alice")); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, ok := c.GetHAProfile("uuid-1")
	if !ok {
		t.Fatalf("expected hit")
	}
	if got.ID != "uuid-1" || got.Name != "alice" {
		t.Errorf("got %+v", got)
	}
	if len(got.Properties) != 1 || got.Properties[0].Name != "textures" {
		t.Errorf("properties round-trip: %+v", got.Properties)
	}
}

func TestHAProfile_Miss(t *testing.T) {
	c := newCache()
	if _, ok := c.GetHAProfile("nope"); ok {
		t.Fatalf("expected miss")
	}
}

func TestMojangProfile_RoundTrip(t *testing.T) {
	c := newCache()
	if err := c.SetMojangProfile("alice", sampleProfile("moj-1", "alice")); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, ok := c.GetMojangProfile("alice")
	if !ok || got.ID != "moj-1" {
		t.Errorf("got ok=%v p=%+v", ok, got)
	}
}

func TestNamespacesAreSeparate(t *testing.T) {
	// HA keyspace uses uuid, Mojang keyspace uses username; they
	// must not collide even if the strings happen to be equal.
	c := newCache()
	_ = c.SetHAProfile("shared", sampleProfile("shared", "ha-name"))
	_ = c.SetMojangProfile("shared", sampleProfile("shared", "moj-name"))
	ha, _ := c.GetHAProfile("shared")
	mj, _ := c.GetMojangProfile("shared")
	if ha.Name != "ha-name" {
		t.Errorf("HA name: %s", ha.Name)
	}
	if mj.Name != "moj-name" {
		t.Errorf("Mojang name: %s", mj.Name)
	}
}

func TestNoop(t *testing.T) {
	n := cache.NewNoop()
	if err := n.SetHAProfile("x", sampleProfile("x", "y")); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, ok := n.GetHAProfile("x"); ok {
		t.Fatalf("expected noop miss")
	}
}

func TestZeroSizeReturnsNil(t *testing.T) {
	if c := cache.NewFreeCache(0, 0); c != nil {
		t.Fatalf("expected nil cache for size=0, got %+v", c)
	}
	if c := cache.NewFreeCache(-1, 0); c != nil {
		t.Fatalf("expected nil cache for size<0, got %+v", c)
	}
}
