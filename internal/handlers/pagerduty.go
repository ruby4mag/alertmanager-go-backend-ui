package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/gin-gonic/gin"
)

// GetPagerDutyServices fetches all PagerDuty services from MongoDB
func GetPagerDutyServices(c *gin.Context) {
	collection := db.GetCollection("pagerduty_services")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cur, err := collection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cur.Close(ctx)

	var services []models.PagerDutyServiceResponse
	for cur.Next(ctx) {
		var dbService models.DbPagerDutyService
		if err := cur.Decode(&dbService); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		
		// Transform to API response format
		services = append(services, models.PagerDutyServiceResponse{
			ID:          dbService.ServiceID,
			Name:        dbService.ServiceName,
			Description: "", // Optional field, can be added to DB model if needed
		})
	}

	// Return empty array instead of null if no services found
	if services == nil {
		services = []models.PagerDutyServiceResponse{}
	}

	c.JSON(http.StatusOK, services)
}

// GetPagerDutyEscalationPolicies fetches all PagerDuty escalation policies from MongoDB
func GetPagerDutyEscalationPolicies(c *gin.Context) {
	collection := db.GetCollection("pagerduty_escalation_policies")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cur, err := collection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cur.Close(ctx)

	var policies []models.PagerDutyEscalationPolicyResponse
	for cur.Next(ctx) {
		var dbPolicy models.DbPagerDutyEscalationPolicy
		if err := cur.Decode(&dbPolicy); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		
		// Transform to API response format
		policies = append(policies, models.PagerDutyEscalationPolicyResponse{
			ID:          dbPolicy.EpID,
			Name:        dbPolicy.EpName,
			Description: "", // Optional field, can be added to DB model if needed
		})
	}

	// Return empty array instead of null if no policies found
	if policies == nil {
		policies = []models.PagerDutyEscalationPolicyResponse{}
	}

	c.JSON(http.StatusOK, policies)
}
