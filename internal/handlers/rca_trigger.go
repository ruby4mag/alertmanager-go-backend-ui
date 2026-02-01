package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TriggerAIRCA builds the RCA graph and sends it to n8n
func TriggerAIRCA(c *gin.Context) {
	alertIDParam := c.Param("id")
	// n8nWebhookURL := "http://n8n:5678/webhook/rca-analysis" // TODO: config
	// For now, we will just build the graph and return it or mock send
	// Check if user requested a dry-run or actual trigger
	dryRun := c.Query("dry_run") == "true"

	objectID, err := primitive.ObjectIDFromHex(alertIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid alert ID format"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Fetch Alert
	alertsCollection := db.GetCollection("alerts")
	var alert models.DbAlert
	err = alertsCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&alert)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
		return
	}
	
	// 2. Build Graph Nodes & Edges
	payload, err := BuildRCAGraph(ctx, alert)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build RCA graph"})
		return
	}
	
	if dryRun {
		c.JSON(http.StatusOK, payload)
		return
	}

	// 4. Send to n8n (Placeholder)
	// Logic to post payload to n8n webhook would go here //
    // For now we just return it to UI to confirm generation
    c.JSON(http.StatusOK, gin.H{"status": "graph_generated", "payload_preview": payload})
}
