package handler

import (
	"net/http"
	"strconv"
	"time"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func RegisterInspirationRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/inspirations", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil { failService(c, err); return }
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
		items, err := svc.ListInspirations(user, c.DefaultQuery("scope", "public"), c.Query("kind"), c.Query("keyword"), limit)
		if err != nil { failService(c, err); return }
		ok(c, gin.H{"items": items})
	})
	r.PATCH("/inspirations/:id/favorite", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil { failService(c, err); return }
		var req struct { Favorite bool `json:"favorite"` }
		if err := c.ShouldBindJSON(&req); err != nil { fail(c, http.StatusBadRequest, err); return }
		if err := svc.SetInspirationFavorite(user, c.Param("id"), req.Favorite); err != nil { failService(c, err); return }
		ok(c, gin.H{"favorite": req.Favorite})
	})
	r.POST("/inspirations/:id/unlock", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil { failService(c, err); return }
		if !enforceRateLimit(c, "inspiration-unlock:"+user.ID, 60, time.Hour) { return }
		purchased, err := svc.UnlockInspiration(user, c.Param("id"))
		if err != nil { failService(c, err); return }
		ok(c, gin.H{"unlocked": true, "purchased": purchased})
	})
	r.POST("/inspirations", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil { failService(c, err); return }
		if !enforceRateLimit(c, "inspiration-create:"+user.ID, 60, time.Hour) { return }
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
		var req service.CreateInspirationRequest
		if err := c.ShouldBindJSON(&req); err != nil { fail(c, http.StatusBadRequest, err); return }
		item, err := svc.CreateInspiration(user, req)
		if err != nil { failService(c, err); return }
		ok(c, gin.H{"item": item})
	})
	r.POST("/inspirations/:id/replicate", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil { failService(c, err); return }
		if !enforceRateLimit(c, "inspiration-replicate:"+user.ID, 120, time.Hour) { return }
		replica, err := svc.ReplicateInspiration(user, c.Param("id"))
		if err != nil { failService(c, err); return }
		ok(c, gin.H{"replica": replica})
	})
}
