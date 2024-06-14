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
	r.GET("/alerts", handlers.Alerts)


	protected := r.Group("/protected")
	protected.Use(auth.AuthMiddleware())
	{
		protected.GET("/resource", handlers.ProtectedResource)
	}

	r.Run(":8080")
}