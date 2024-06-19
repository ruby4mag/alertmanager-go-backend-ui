package main

import (
	"time"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/auth"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		//AllowAllOrigins: true,
		AllowOrigins: []string{"http://192.168.1.201:3000"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin","Authorization","X-Requested-With", "Content-Type", "Accept"},
		ExposeHeaders: []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge: 12 * time.Hour,
	}))

	r.POST("/register", handlers.Register)
	r.POST("/login", handlers.Login)
	r.POST("/refresh", handlers.RefreshToken)

	protected := r.Group("/api")
	protected.Use(auth.AuthMiddleware())
	{
		protected.GET("/alertrules", handlers.Index)
		protected.POST("/alertrules", handlers.New)
		protected.GET("/alertrules/:id", handlers.Edit)
		protected.PUT("/alertrules/:id", handlers.Update)

		protected.GET("/alerts", handlers.Alerts)
		protected.GET("/alerts/:id", handlers.View)
		protected.POST("/alerts/:id/comment", handlers.AddComment)

		protected.GET("/resource", handlers.ProtectedResource)
	}

	r.Run(":8080")
}
