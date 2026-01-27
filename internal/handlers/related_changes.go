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
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetRelatedChanges returns changes related to an alert based on entity and time
func GetRelatedChanges(c *gin.Context) {
	alertIDParam := c.Param("alert_id")
	
	// Convert alertID to ObjectID
	objectID, err := primitive.ObjectIDFromHex(alertIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid alert ID format"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Fetch the Alert
	alertsCollection := db.GetCollection("alerts")
	var alert models.DbAlert
	err = alertsCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&alert)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
		return
	}

	// 2. Define Query Parameters
	entityID := alert.Entity
	alertStartTime := alert.AlertFirstTime.Time
	alertEndTime := alert.AlertClearTime.Time
	
	// If AlertClearTime is zero (active alert), consider "now" as the end boundary for overlap logic?
	// Requirement: change.start_time <= alert_end_time.
	// If the alert is active, it technically hasn't ended. But for a "related changes" query, 
	// we usually want changes that happened *up to now*.
	// However, if we strictly follow "change.start_time <= alert_end_time", and alert_end_time is Zero,
	// we need to decide what to use. 
	// Let's assume if ClearTime is zero, we use Now() for the purpose of finding *started* changes.
	
	effectiveEndTime := alertEndTime
	if effectiveEndTime.IsZero() {
		effectiveEndTime = time.Now()
	}

	// 3. Build Changes Filter
	// Entity match: affected_entities contains alert's entity
	// Time overlap:
	//   change.start_time <= effectiveEndTime
	//   AND (change.end_time is null OR change.end_time >= alert.start_time)
	// Status filter: scheduled, in_progress, completed
	
	filter := bson.M{
		"affected_entities": entityID,
		"status": bson.M{"$in": []string{"scheduled", "in_progress", "completed"}},
		"start_time": bson.M{"$lte": effectiveEndTime},
		"$or": []bson.M{
			{"end_time": nil}, // Start of time or open-ended
			{"end_time": bson.M{"$exists": false}},
			{"end_time": bson.M{"$gte": alertStartTime}},
		},
	}

	// 4. Query Changes
	changesCollection := db.GetCollection("changes")
	findOptions := options.Find().SetSort(bson.D{{Key: "start_time", Value: -1}}) // Newest first

	cursor, err := changesCollection.Find(ctx, filter, findOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query related changes"})
		return
	}
	defer cursor.Close(ctx)

	var changes []models.Change
	if err := cursor.All(ctx, &changes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode changes"})
		return
	}

	// 5. Construct Response
	relatedChanges := []models.RelatedChange{}
	for _, ch := range changes {
		// Overlap classification
		overlapType := "during_alert"
		if ch.StartTime.Before(alertStartTime) {
			overlapType = "before_alert"
		}
		// Note regarding "after_alert": 
		// Since we filter by change.start_time <= effectiveEndTime, 
		// a change cannot start after the alert ends (or now).
		// So "after_alert" is not reachable under strict query rules unless "after" means "after start" (which matches "during").
		// We will stick to before/during distinction relative to Alert Start.

		relatedChanges = append(relatedChanges, models.RelatedChange{
			ChangeID:      ch.ChangeID,
			Name:          ch.Name,
			ChangeType:    ch.ChangeType,
			Status:        ch.Status,
			ImplementedBy: ch.ImplementedBy,
			StartTime:     ch.StartTime,
			EndTime:       ch.EndTime,
			OverlapType:   overlapType,
		})
	}

	response := models.RelatedChangesResponse{
		AlertID:        alert.AlertId, // User friendly ID
		EntityID:       alert.Entity,
		RelatedChanges: relatedChanges,
	}
	
	// Fallback if AlertId is empty, use ObjectID
	if response.AlertID == "" {
		response.AlertID = alert.ID.Hex()
	}

	c.JSON(http.StatusOK, response)
}
