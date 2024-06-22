package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/auth"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"

	"github.com/gin-gonic/gin"
)

var ctx = context.Background()

func Register(c *gin.Context) {
    var request struct {
        Username string `json:"username"`
		Email string `json:"email"`
		Role	string  `json:"role"`
        Password string `json:"password"`
    }

    if err := c.BindJSON(&request); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }

    existingUser, err := models.FindUserByUsername(request.Username)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
        return
    }
    if existingUser != nil {
        c.JSON(http.StatusConflict, gin.H{"error": "Username already taken"})
        return
    }

    user := models.User{
        Username: request.Username,
		Email: request.Email,
		Role:  request.Role,
    }

    if err := user.HashPassword(request.Password); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
        return
    }

    if err := models.CreateUser(&user); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
        return
    }

    c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
}

func Login(c *gin.Context) {
    var credentials struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }

    if err := c.BindJSON(&credentials); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }

    user, err := models.FindUserByUsername(credentials.Username)
    fmt.Println(user)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
        return
    }
    if user == nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
        return
    }

    if err := user.CheckPassword(credentials.Password); err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
        return
    }

    jwtToken, err := auth.GenerateJWT(user.Username)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
        return
    }
    refreshToken, err := auth.GenerateRefreshToken()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
        return
    }

    db.RedisClient.Set(ctx, refreshToken, user.Username, 24*time.Hour)

    c.JSON(http.StatusOK, gin.H{
        "token":         jwtToken,
        "refresh_token": refreshToken,
        "username" : "Magin",
        "role" : user.Role,
    })
}

func RefreshToken(c *gin.Context) {
	var request struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	username, err := db.RedisClient.Get(ctx, request.RefreshToken).Result()
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	newToken, err := auth.GenerateJWT(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": newToken})
}

func ProtectedResource(c *gin.Context) {
	username := c.MustGet("username").(string)
	c.JSON(http.StatusOK, gin.H{"message": "Hello " + username})
}

func GetPermissions(c *gin.Context){
    permissions := make(map[string]interface{})
    permissions["admin"] = []string{"NotifyRuleEdit", "AlertRuleEdit","TagRuleEdit"}
    permissions["user"] = []string{}
    c.JSON(http.StatusOK, gin.H{"permissions": permissions })
}