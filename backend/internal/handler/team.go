package handler

import (
	"github.com/gin-gonic/gin"
	"infinite-canvas/backend/internal/service"
	"net/http"
)

func RegisterTeamRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/teams", func(c *gin.Context) {
		u, e := currentUser(c, svc)
		if e != nil {
			failService(c, e)
			return
		}
		items, e := svc.Teams(u)
		if e != nil {
			failService(c, e)
			return
		}
		ok(c, gin.H{"items": items})
	})
	r.POST("/teams", func(c *gin.Context) {
		u, e := currentUser(c, svc)
		if e != nil {
			failService(c, e)
			return
		}
		var req service.CreateTeamRequest
		if c.ShouldBindJSON(&req) != nil {
			fail(c, http.StatusBadRequest, service.BadAuthRequest("团队信息格式无效"))
			return
		}
		item, e := svc.CreateTeam(u, req)
		if e != nil {
			failService(c, e)
			return
		}
		ok(c, gin.H{"team": item})
	})
	r.POST("/teams/join", func(c *gin.Context) {
		u, e := currentUser(c, svc)
		if e != nil {
			failService(c, e)
			return
		}
		var req struct {
			Code string `json:"code"`
		}
		if c.ShouldBindJSON(&req) != nil {
			fail(c, http.StatusBadRequest, service.BadAuthRequest("请输入团队邀请码"))
			return
		}
		team, joined, e := svc.JoinTeam(u, req.Code)
		if e != nil {
			failService(c, e)
			return
		}
		ok(c, gin.H{"team": team, "joined": joined})
	})
	r.GET("/teams/:id", func(c *gin.Context) {
		u, e := currentUser(c, svc)
		if e != nil {
			failService(c, e)
			return
		}
		item, e := svc.TeamDetail(u, c.Param("id"))
		if e != nil {
			failService(c, e)
			return
		}
		ok(c, item)
	})
	r.POST("/teams/:id/leave", func(c *gin.Context) {
		u, e := currentUser(c, svc)
		if e != nil {
			failService(c, e)
			return
		}
		if e = svc.LeaveTeam(u, c.Param("id")); e != nil {
			failService(c, e)
			return
		}
		ok(c, gin.H{"left": true})
	})
	r.GET("/teams/:id/resources", func(c *gin.Context) {
		u, e := currentUser(c, svc)
		if e != nil {
			failService(c, e)
			return
		}
		items, e := svc.TeamResources(u, c.Param("id"), c.Query("type"))
		if e != nil {
			failService(c, e)
			return
		}
		ok(c, gin.H{"items": items})
	})
	r.POST("/teams/:id/resources", func(c *gin.Context) {
		u, e := currentUser(c, svc)
		if e != nil {
			failService(c, e)
			return
		}
		var req service.ShareTeamResourceRequest
		if c.ShouldBindJSON(&req) != nil {
			fail(c, http.StatusBadRequest, service.BadAuthRequest("共享资源信息无效"))
			return
		}
		item, e := svc.ShareTeamResource(u, c.Param("id"), req)
		if e != nil {
			failService(c, e)
			return
		}
		ok(c, gin.H{"item": item})
	})
	r.GET("/teams/:id/resources/:type/:sourceId/content", func(c *gin.Context) {
		u, e := currentUser(c, svc)
		if e != nil {
			failService(c, e)
			return
		}
		content, e := svc.TeamResourceContent(u, c.Param("id"), c.Param("type"), c.Param("sourceId"))
		if e != nil {
			failService(c, e)
			return
		}
		ok(c, gin.H{"content": content})
	})
	r.DELETE("/teams/:id/resources/:type/:sourceId", func(c *gin.Context) {
		u, e := currentUser(c, svc)
		if e != nil {
			failService(c, e)
			return
		}
		if e = svc.UnshareTeamResource(u, c.Param("id"), c.Param("type"), c.Param("sourceId")); e != nil {
			failService(c, e)
			return
		}
		ok(c, gin.H{"deleted": true})
	})
}
