package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/winnerproxy/winnerproxy/internal/cache"
)

// Handler bundles the dependencies needed by HTTP handlers.
type Handler struct {
	Cache *cache.Cache
}

// New constructs a Handler.
func New(c *cache.Cache) *Handler {
	return &Handler{Cache: c}
}

// Health is a simple liveness probe.
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// CacheGet returns the cached value for :key, or 404 on miss.
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

// CacheSetRequest is the JSON body accepted by CacheSet.
type CacheSetRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value" binding:"required"`
	// TTLSeconds == 0 means "never expire".
	TTLSeconds int `json:"ttl_seconds"`
}

// CacheSet stores a value in the cache.
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

// CacheDelete removes a value from the cache.
func (h *Handler) CacheDelete(c *gin.Context) {
	key := []byte(c.Param("key"))
	deleted := h.Cache.Delete(key)
	c.JSON(http.StatusOK, gin.H{"key": string(key), "deleted": deleted})
}

// CacheStats returns cache hit/miss statistics.
func (h *Handler) CacheStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.Cache.Stats())
}

func expiryTime(ttlSeconds int) string {
	if ttlSeconds <= 0 {
		return "never"
	}
	return time.Now().Add(time.Duration(ttlSeconds) * time.Second).UTC().Format(time.RFC3339)
}
