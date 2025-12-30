package main

import (
	"time"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/auth"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)



func main() {
	const NoderedEndpoint = "http://192.168.1.201:1880/notifications"
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		//AllowOrigins: []string{"http://192.168.1.201:3000"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin","Authorization","X-Requested-With", "Content-Type", "Accept"},
		ExposeHeaders: []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge: 12 * time.Hour,
	}))



	r.POST("/register", handlers.Register)
	r.POST("/login", handlers.Login)
	r.POST("/refresh", handlers.RefreshToken)
	r.POST("/alerts/callback", handlers.AlertCallback)
	r.GET("/entity/:name", handlers.HandleEntityGraph)

	protected := r.Group("/api")
	protected.Use(auth.AuthMiddleware())
	{
		protected.GET("/permissions", handlers.GetPermissions)
		
		protected.GET("/alertrules", handlers.Index)
		protected.POST("/alertrules", handlers.New)
		protected.GET("/alertrules/:id", handlers.Edit)
		protected.PUT("/alertrules/:id", handlers.Update)

		protected.GET("/healrules", handlers.IndexHeal)
		protected.POST("/healrules", handlers.NewHeal)
		protected.GET("/healrules/:id", handlers.EditHeal)
		protected.PUT("/healrules/:id", handlers.UpdateHeal)

		protected.GET("/notifyrules", handlers.IndexNotify)
		protected.POST("/notifyrules", handlers.NewNotify)
		protected.GET("/notifyrules/:id", handlers.EditNotify)
		protected.PUT("/notifyrules/:id", handlers.UpdateNotify)

		protected.GET ("/tagrules", handlers.IndexTag)
		protected.POST("/tagrules", handlers.NewTag)
		protected.GET("/tagrules/:id", handlers.EditTag)
		protected.PUT("/tagrules/:id", handlers.UpdateTag)

		protected.GET("/correlationrules", handlers.IndexCorrelation)
		protected.POST("/correlationrules", handlers.NewCorrelation)
		protected.GET("/correlationrules/:id", handlers.EditCorrelation)
		protected.PUT("/correlationrules/:id", handlers.UpdateCorrelation)

		protected.GET("/alerts", handlers.Alerts)
		protected.POST("/alerts/:id/notify/:notificationid", handlers.Notify)

		protected.GET("/alerts/:id", handlers.View)
		protected.POST("/alerts/:id/comment", handlers.AddComment)
		protected.POST("/alerts/:id/acknowledge", handlers.Acknowledge)
		protected.POST("/alerts/:id/unacknowledge", handlers.Unacknowledge)
		protected.POST("/alerts/:id/clear", handlers.Clear)

		protected.GET("/resource", handlers.ProtectedResource)
	}

	r.Run(":8080")
}
