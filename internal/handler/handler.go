package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/winnerproxy/winnerproxy/internal/cache"
	"github.com/winnerproxy/winnerproxy/internal/mapping"
	"github.com/winnerproxy/winnerproxy/internal/proxy"
)

var (
	ErrMultiAuth = errors.New("multiple services returned profiles")
)

type Handler struct {
	Cache   *cache.Cache
	Proxy   *proxy.Proxy
	Mapping *mapping.Mapping
}

func New(c *cache.Cache, p *proxy.Proxy, m *mapping.Mapping) *Handler {
	return &Handler{Cache: c, Proxy: p, Mapping: m}
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) CacheGet(c *gin.Context) {
	key := []byte(c.Param("key"))
	val, err := h.Cache.Get(key)
	if err != nil {
		if errors.Is(err, cache.ErrCacheMiss) {
			c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", val)
}

type CacheSetRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value" binding:"required"`
	TTLSeconds int `json:"ttl_seconds"`
}

func (h *Handler) CacheSet(c *gin.Context) {
	var req CacheSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Cache.Set([]byte(req.Key), []byte(req.Value), req.TTLSeconds); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"key":         req.Key,
		"ttl_seconds": req.TTLSeconds,
		"expires_at":  expiryTime(req.TTLSeconds),
	})
}

func (h *Handler) CacheDelete(c *gin.Context) {
	key := []byte(c.Param("key"))
	deleted := h.Cache.Delete(key)
	c.JSON(http.StatusOK, gin.H{"key": string(key), "deleted": deleted})
}

func (h *Handler) CacheStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.Cache.Stats())
}

func (h *Handler) HasJoined(c *gin.Context) {
	params := url.Values{}
	for k, v := range c.Request.URL.Query() {
		params[k] = v
	}

	services := h.Proxy.GetServices()
	if len(services) == 0 {
		c.Status(http.StatusNoContent)
		return
	}

	var results []struct {
		Service *proxy.UpstreamService
		Profile *proxy.PlayerProfile
	}

	for _, service := range services {
		profile, err := service.HasJoined(params)
		if err != nil {
			if errors.Is(err, proxy.ErrNoProfile) {
				continue
			}
			continue
		}
		results = append(results, struct {
			Service *proxy.UpstreamService
			Profile *proxy.PlayerProfile
		}{&service, profile})
	}

	if len(results) == 0 {
		c.Status(http.StatusNoContent)
		return
	}

	if len(results) > 1 {
		c.JSON(http.StatusConflict, gin.H{"error": ErrMultiAuth.Error()})
		return
	}

	result := results[0]
	transformed, err := h.Mapping.Transform(*result.Service, result.Profile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transformed)
}

func (h *Handler) QueryProfile(c *gin.Context) {
	uuid := c.Param("uuid")
	unsigned := true
	if v := c.Query("unsigned"); v != "" {
		unsigned, _ = strconv.ParseBool(v)
	}

	mappingData, err := h.Mapping.QueryByDownstreamUUID(uuid)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	services := h.Proxy.GetServices()
	var service proxy.UpstreamService
	for _, s := range services {
		if s.ID() == mappingData.DeclaredYggdrasilTree {
			service = s
			break
		}
	}

	if service == nil {
		c.Status(http.StatusNotFound)
		return
	}

	profile, err := service.QueryProfile(mappingData.UpstreamUUID, unsigned)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, &proxy.PlayerProfile{
		ID:         mappingData.DownstreamUUID,
		Name:       mappingData.DownstreamName,
		Properties: profile.Properties,
	})
}

func (h *Handler) BatchQuery(c *gin.Context) {
	var names []string
	if err := json.NewDecoder(c.Request.Body).Decode(&names); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type profileResult struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var results []profileResult
	for _, name := range names {
		services := h.Proxy.GetServices()
		for _, service := range services {
			profiles, err := service.BatchQuery([]string{name})
			if err != nil || len(profiles) == 0 {
				continue
			}
			transformed, err := h.Mapping.Transform(service, profiles[0])
			if err != nil {
				continue
			}
			results = append(results, profileResult{
				ID:   transformed.ID,
				Name: transformed.Name,
			})
			break
		}
	}

	c.JSON(http.StatusOK, results)
}

func (h *Handler) YggdrasilRoot(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"skinDomains": []string{},
	})
}

func expiryTime(ttlSeconds int) string {
	if ttlSeconds <= 0 {
		return "never"
	}
	return time.Now().Add(time.Duration(ttlSeconds) * time.Second).UTC().Format(time.RFC3339)
}
