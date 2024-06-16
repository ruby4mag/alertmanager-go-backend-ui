package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"

	"github.com/gin-gonic/gin"
)



func New(c *gin.Context) {
    

    var alertRule  models.DbAlertRule
    collection := db.GetCollection("alertrules")

    if err := c.ShouldBindJSON(&alertRule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := collection.InsertOne(ctx, alertRule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": result.InsertedID})
}


