package router

import (
	"github.com/gin-gonic/gin"

	"github.com/winnerproxy/winnerproxy/internal/handler"
)

// New builds the Gin engine with all routes registered.
func New(h *handler.Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/health", h.Health)

	cacheGroup := r.Group("/cache")
	{
		cacheGroup.GET("/:key", h.CacheGet)
		cacheGroup.POST("", h.CacheSet)
		cacheGroup.DELETE("/:key", h.CacheDelete)
		cacheGroup.GET("/stats", h.CacheStats)
	}

	return r
}
