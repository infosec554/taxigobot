package bot

import (
	"context"
	"net/http"
	"taxibot/pkg/logger"
	"taxibot/storage"

	"github.com/gin-gonic/gin"
)

func RunServer(stg storage.IStorage, log logger.ILogger) error {
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
	}

	return r.Run(":8080")
}
