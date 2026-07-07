package router

import (
	"github.com/gin-gonic/gin"

	"github.com/winnerproxy/winnerproxy/internal/handler"
)

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

	yggdrasilGroup := r.Group("/yggdrasil")
	{
		yggdrasilGroup.GET("", h.YggdrasilRoot)
		yggdrasilGroup.GET("/sessionserver/session/minecraft/hasJoined", h.HasJoined)
		yggdrasilGroup.GET("/sessionserver/session/minecraft/profile/:uuid", h.QueryProfile)
		yggdrasilGroup.POST("/api/profiles/minecraft", h.BatchQuery)
	}

	return r
}
