package bot

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"taxibot/config"
	"taxibot/pkg/logger"
	"taxibot/storage"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

func RunServer(cfg *config.Config, stg storage.IStorage, log logger.ILogger, notifySuccess func(int64)) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Static files for Mini App
	r.Static("/web", "./web")

	// API Endpoints
	api := r.Group("/api")
	{
		api.GET("/orders/active", func(c *gin.Context) {
			orders, err := stg.Order().GetActiveOrders(context.Background())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, orders)
		})

		api.GET("/locations", func(c *gin.Context) {
			locations, err := stg.Location().GetAll(context.Background())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, locations)
		})

		api.POST("/payments/webhook", func(c *gin.Context) {
			// Read body for signature verification
			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				log.Error("Failed to read webhook body", logger.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
				return
			}

			// Verify signature if API Secret is set
			if cfg.CPAPISecret != "" {
				signature := c.GetHeader("X-Content-HMAC")
				if signature == "" {
					log.Warning("Missing X-Content-HMAC header")
					c.JSON(http.StatusUnauthorized, gin.H{"error": "missing signature"})
					return
				}

				mac := hmac.New(sha256.New, []byte(cfg.CPAPISecret))
				mac.Write(body)
				expectedMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))

				if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
					log.Warning("Invalid webhook signature", logger.String("received", signature), logger.String("expected", expectedMAC))
					c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
					return
				}
			}

			// Restore body for binding
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

			// CloudPayments sends payment details in the body
			var payload struct {
				OrderID int64  `json:"InvoiceId"`
				Amount  int    `json:"Amount"`
				Status  string `json:"Status"`
			}

			if err := c.ShouldBindJSON(&payload); err != nil {
				// Also support form-urlencoded if needed
				invoiceID := cast.ToInt64(c.PostForm("InvoiceId"))
				if invoiceID != 0 {
					payload.OrderID = invoiceID
					payload.Status = c.PostForm("Status")
				} else {
					log.Error("Failed to bind payment webhook", logger.Error(err))
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
					return
				}
			}

			log.Info("Received payment webhook", logger.Int64("order_id", payload.OrderID), logger.String("status", payload.Status))

			// Typically Status "Completed" or "Authorized" means success
			if payload.Status == "Completed" || payload.Status == "Authorized" {
				// Update order status to active
				err := stg.Order().UpdateStatus(context.Background(), payload.OrderID, "active")
				if err != nil {
					log.Error("Failed to update order status after payment", logger.Error(err))
					c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
					return
				}

				// Trigger notifications via bot peer
				// We call a new method on Bot to handle this notification
				// We need access to the bot instance here.
				// Since stg is passed, but not Bot, we might need to pass Bot or a notify function.
				// For now, let's assume we can trigger a notification if we have access to the Bot.
				// Actually, RunServer is called from main.go where Bot is created.
				notifySuccess(payload.OrderID)
			}

			c.JSON(http.StatusOK, gin.H{"code": 0})
		})
	}

	return r.Run(fmt.Sprintf(":%d", cfg.AppPort))
}
