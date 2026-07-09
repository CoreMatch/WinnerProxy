package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
	"github.com/winnerproxy/winnerproxy/internal/mapping"
	"github.com/winnerproxy/winnerproxy/internal/proxy"
)

// Handler dispatches Yggdrasil requests. HasJoined follows the
// three-stage flow (HRPAuth → Mojang → proxy-register); QueryProfile /
// BatchQuery / YggdrasilRoot go directly to HRPAuth. Services and
// Mapping are kept as fields only so P4's mapping-removal commit can
// touch them in isolation.
type Handler struct {
	Hrpauth  *hrpauth.Client
	Services []proxy.UpstreamService
	Mapping  *mapping.Mapping
}

// New constructs a Handler. hrpauthCli is used for the three-stage
// HasJoined and the direct HRPAuth calls in QueryProfile / BatchQuery /
// YggdrasilRoot. services is used to locate the MojangService for
// stage 2. m is held for P4 cleanup.
func New(services []proxy.UpstreamService, hrpauthCli *hrpauth.Client, m *mapping.Mapping) *Handler {
	return &Handler{Hrpauth: hrpauthCli, Services: services, Mapping: m}
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// HasJoined is the entry point for the Yggdrasil hasJoined query. The
// query is passed through verbatim to HA so any future HA-side
// parameters (e.g. ip) work without WinnerProxy changes.
//
// Three-stage flow:
//  1. HA hasJoined — if HA returns 200, return HA's profile (HA skin).
//  2. Mojang hasJoined — if Mojang returns 200, take the profile.
//  3. Proxy-register — call HA POST /register with M.T. + Mojang UUID,
//     return HA's profile_id as the player identity with Mojang skin.
func (h *Handler) HasJoined(c *gin.Context) {
	params := url.Values{}
	for k, v := range c.Request.URL.Query() {
		params[k] = v
	}

	// Stage 1: HA auth path.
	profile, err := h.Hrpauth.HasJoined(params)
	if err == nil && profile != nil {
		c.JSON(http.StatusOK, profile)
		return
	}
	if err != nil && !errors.Is(err, hrpauth.ErrNoProfile) {
		// HA 5xx / network — log and continue to Mojang fallback.
		log.Printf("hrpauth hasJoined error, falling back to mojang: %v", err)
	}

	// Stage 2: Mojang auth path.
	mojang := h.findMojang()
	if mojang == nil {
		c.Status(http.StatusNoContent)
		return
	}
	mojangProfile, err := mojang.HasJoined(params)
	if err != nil || mojangProfile == nil {
		c.Status(http.StatusNoContent)
		return
	}

	// Stage 3: proxy-register in HA.
	password, err := generatePassword()
	if err != nil {
		log.Printf("generate password failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth backend unavailable"})
		return
	}
	reg, err := h.Hrpauth.RegisterByProxy(mojangProfile.Name, mojangProfile.ID, password)
	switch {
	case errors.Is(err, hrpauth.ErrUsernameBound):
		log.Printf("username_already_bound, rejecting mojang player: name=%s uuid=%s",
			mojangProfile.Name, mojangProfile.ID)
		c.Status(http.StatusNoContent)
	case err != nil:
		log.Printf("hrpauth register failed: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth backend unavailable"})
	default:
		// Stage 3 success: HA identity (profile_id) + Mojang skin.
		c.JSON(http.StatusOK, &hrpauth.PlayerProfile{
			ID:         reg.ProfileID,
			Name:       mojangProfile.Name,
			Properties: mojangProfile.Properties,
		})
	}
}

// findMojang returns the MojangService from the upstream list, or nil
// if the official Mojang upstream is disabled.
func (h *Handler) findMojang() proxy.UpstreamService {
	for _, s := range h.Services {
		if s.ID() == "mojang" {
			return s
		}
	}
	return nil
}

// generatePassword returns a 16-byte random password encoded as
// base64-URL (22 chars). Players never use this — HA only stores it.
func generatePassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// QueryProfile fetches a profile by UUID directly from HRPAuth.
func (h *Handler) QueryProfile(c *gin.Context) {
	uuid := c.Param("uuid")
	unsigned := true
	if v := c.Query("unsigned"); v != "" {
		unsigned, _ = strconv.ParseBool(v)
	}
	profile, err := h.Hrpauth.GetProfile(uuid, unsigned)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, profile)
}

// BatchQuery forwards a list of usernames directly to HRPAuth. Only
// {id, name} summary is returned; textures properties are dropped per
// Yggdrasil convention.
func (h *Handler) BatchQuery(c *gin.Context) {
	var names []string
	if err := json.NewDecoder(c.Request.Body).Decode(&names); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	profiles, err := h.Hrpauth.BatchQuery(names)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type profileResult struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	out := make([]profileResult, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, profileResult{ID: p.ID, Name: p.Name})
	}
	c.JSON(http.StatusOK, out)
}

// YggdrasilRoot is the meta endpoint served at "/" and "/yggdrasil".
// It passes through HA's root response so the Minecraft client gets
// HA's real skinDomains / signaturePublickey. On HA failure it
// returns a minimal {skinDomains: []} so legacy probes still succeed.
func (h *Handler) YggdrasilRoot(c *gin.Context) {
	meta, err := h.Hrpauth.GetServerMeta()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"skinDomains": []string{}})
		return
	}
	c.JSON(http.StatusOK, meta)
}
