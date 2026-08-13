package handler

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

const maxPaymentWebhookBody = 2 << 20

func RegisterPaymentRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/payments/catalog", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		catalog, err := svc.PaymentCatalog(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"catalog": catalog})
	})

	r.POST("/payments/orders", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if !enforceRateLimit(c, "payment-create:"+user.ID, 20, time.Hour) {
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
		var req service.CreatePaymentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		order, err := svc.CreatePayment(c.Request.Context(), user, req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"order": order})
	})

	r.GET("/payments/orders", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		orders, err := svc.PaymentOrders(user, 20)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"orders": orders})
	})

	r.GET("/payments/orders/:id", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		order, err := svc.PaymentOrder(user, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"order": order})
	})

	r.GET("/payments/paypal/return", func(c *gin.Context) {
		localOrderID := strings.TrimSpace(c.Query("order"))
		providerOrderID := strings.TrimSpace(c.Query("token"))
		order, err := svc.CapturePayPalOrder(c.Request.Context(), localOrderID, providerOrderID)
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/wallet?paymentOrder="+url.QueryEscape(localOrderID)+"&paymentResult=failed")
			return
		}
		c.Redirect(http.StatusSeeOther, "/wallet?paymentOrder="+url.QueryEscape(order.ID)+"&paymentResult=paid")
	})

	r.POST("/payments/webhooks/alipay", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)
		if err := c.Request.ParseForm(); err != nil || svc.HandleAlipayWebhook(c.Request.PostForm) != nil {
			c.String(http.StatusBadRequest, "failure")
			return
		}
		c.String(http.StatusOK, "success")
	})

	r.POST("/payments/webhooks/wechat", func(c *gin.Context) {
		body, err := readPaymentWebhookBody(c)
		if err != nil || svc.HandleWeChatWebhook(c.Request.Header, body) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "invalid notification"})
			return
		}
		c.Status(http.StatusNoContent)
	})

	r.POST("/payments/webhooks/stripe", func(c *gin.Context) {
		body, err := readPaymentWebhookBody(c)
		if err != nil || svc.HandleStripeWebhook(c.GetHeader("Stripe-Signature"), body) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"received": true})
	})

	r.POST("/payments/webhooks/paypal", func(c *gin.Context) {
		body, err := readPaymentWebhookBody(c)
		if err != nil || svc.HandlePayPalWebhook(c.Request.Context(), c.Request.Header, body) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook"})
			return
		}
		c.Status(http.StatusNoContent)
	})

	r.POST("/payments/webhooks/generic", func(c *gin.Context) {
		body, err := readPaymentWebhookBody(c)
		if err != nil || svc.HandleGenericWebhook(c.GetHeader("X-Payment-Signature"), c.GetHeader("X-Payment-Timestamp"), body) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook"})
			return
		}
		c.Status(http.StatusNoContent)
	})

	r.GET("/admin/settings/payments", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		setting, err := svc.AdminPaymentSetting(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"setting": setting})
	})

	r.PATCH("/admin/settings/payments", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		var req service.PaymentSetting
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		setting, err := svc.UpdatePaymentSetting(user, req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"setting": setting})
	})
}

func readPaymentWebhookBody(c *gin.Context) ([]byte, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPaymentWebhookBody)
	return io.ReadAll(c.Request.Body)
}
