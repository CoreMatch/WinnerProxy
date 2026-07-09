package router

import (
	"github.com/gin-gonic/gin"

	"github.com/winnerproxy/winnerproxy/internal/handler"
)

func New(h *handler.Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/health", h.Health)

	// Yggdrasil status endpoint: serve at both "/" and "/yggdrasil"
	// so Mojang/legacy clients that probe the root path get a valid response.
	r.GET("/", h.YggdrasilRoot)
	r.GET("/yggdrasil", h.YggdrasilRoot)

	// Register every Yggdrasil route under both the prefix-less
	// (Mojang-style) path and the "/yggdrasil" prefix. Minecraft
	// launchers and legacy probes hit the prefix-less form, while
	// HA's own clients use the prefixed form. The handler is
	// identical, so we just bind the same function twice.
	bindYggdrasil := func(group *gin.RouterGroup) {
		group.GET("/sessionserver/session/minecraft/hasJoined", h.HasJoined)
		group.GET("/sessionserver/session/minecraft/profile/:uuid", h.QueryProfile)
		group.POST("/api/profiles/minecraft", h.BatchQuery)
	}
	bindYggdrasil(r.Group("/"))
	bindYggdrasil(r.Group("/yggdrasil"))

	return r
}
