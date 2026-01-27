package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NeighborNode represents a discovered neighbor
type NeighborNode struct {
	EntityID string
	Distance int
}

// GetRelatedChanges returns changes related to an alert (direct + neighbors)
func GetRelatedChanges(c *gin.Context) {
	alertIDParam := c.Param("alert_id")
	
	objectID, err := primitive.ObjectIDFromHex(alertIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid alert ID format"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) 
	defer cancel()

	// 1. Fetch the Alert
	alertsCollection := db.GetCollection("alerts")
	var alert models.DbAlert
	err = alertsCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&alert)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
		return
	}

	rootEntityID := alert.Entity
	alertStartTime := alert.AlertFirstTime.Time
	alertEndTime := alert.AlertClearTime.Time
	effectiveEndTime := alertEndTime
	if effectiveEndTime.IsZero() {
		effectiveEndTime = time.Now()
	}

	// 2. Discover Neighbors via Neo4j
	neighborsMap := make(map[string]int) // entity -> min_distance
	
	// We only need neighbors if we can connect to Neo4j.
	// If Neo4j is down or empty, we fallback to just direct changes gracefully.
	neo4jDriver := db.GetNeo4jDriver()
	if neo4jDriver != nil {
		session := neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		// Cypher: Find neighbors up to 6 hops
		// Return: neighbor name, min(distance)
		cypherQuery := `
		MATCH (root)
		WHERE root.name = $rootName OR root.id = $rootName
		MATCH p = (root)-[*1..6]-(neighbor)
		WITH neighbor, length(p) as distance
		ORDER BY distance ASC
		RETURN neighbor.name as name, min(distance) as dist
		LIMIT 500
		`
		
		res, err := session.Run(ctx, cypherQuery, map[string]interface{}{"rootName": rootEntityID})
		if err == nil {
			for res.Next(ctx) {
				rec := res.Record()
				name, ok1 := rec.Get("name")
				dist, ok2 := rec.Get("dist")
				
				if ok1 && ok2 {
					entityName := name.(string)
					distance := int(dist.(int64))
					// Only add if not already present or found closer path (though query orders by distance)
					if _, exists := neighborsMap[entityName]; !exists {
						neighborsMap[entityName] = distance
					}
				}
			}
		} else {
			log.Printf("Neo4j neighbor query failed: %v", err)
		}
	} else {
        log.Println("Neo4j driver is nil, skipping neighbor discovery")
    }

	// 3. Define Change Filter (Base)
	// Status: scheduled, in_progress, completed
	// Time: Overlaps with alert
	baseFilter := bson.M{
		"status": bson.M{"$in": []string{"scheduled", "in_progress", "completed"}},
		"start_time": bson.M{"$lte": effectiveEndTime},
		"$or": []bson.M{
			{"end_time": nil},
			{"end_time": bson.M{"$exists": false}},
			{"end_time": bson.M{"$gte": alertStartTime}},
		},
	}

	changesCollection := db.GetCollection("changes")
	findOptions := options.Find().SetSort(bson.D{{Key: "start_time", Value: -1}})

	// 4. Query & Process Direct Changes
	directFilter := cloneMap(baseFilter)
	directFilter["affected_entities"] = rootEntityID
	
	var directChanges []models.RelatedChange
	
	cursorDirect, err := changesCollection.Find(ctx, directFilter, findOptions)
	if err == nil {
		var rawDirect []models.Change
		if err := cursorDirect.All(ctx, &rawDirect); err == nil {
			for _, ch := range rawDirect {
				directChanges = append(directChanges, mapChange(ch, rootEntityID, 0, "direct", alertStartTime))
			}
		}
	} else {
        log.Printf("Error querying direct changes: %v", err)
    }

	// 5. Query & Process Neighbor Changes
	var neighborChanges []models.RelatedChange
	
	if len(neighborsMap) > 0 {
		neighborEntities := make([]string, 0, len(neighborsMap))
		for k := range neighborsMap {
			neighborEntities = append(neighborEntities, k)
		}
		
		neighborFilter := cloneMap(baseFilter)
		neighborFilter["affected_entities"] = bson.M{"$in": neighborEntities}
		
		cursorNeighbor, err := changesCollection.Find(ctx, neighborFilter, findOptions)
		if err == nil {
			var rawNeighbor []models.Change
			if err := cursorNeighbor.All(ctx, &rawNeighbor); err == nil {
				for _, ch := range rawNeighbor {
					// Identify which specific neighbor(s) triggers this. 
					// A change might affect multiple entities. We pick the impacted neighbor with smallest distance.
					bestHop := 100
					bestEntity := ""
					
					// Check overlap between change.AffectedEntities and neighborsMap
					foundMatch := false
					for _, affected := range ch.AffectedEntities {
						if dist, ok := neighborsMap[affected]; ok {
							if dist < bestHop {
								bestHop = dist
								bestEntity = affected
								foundMatch = true
							}
						}
					}
					
					if foundMatch {
						neighborChanges = append(neighborChanges, mapChange(ch, bestEntity, bestHop, "neighbor", alertStartTime))
					}
				}
			}
		} else {
            log.Printf("Error querying neighbor changes: %v", err)
        }
	}

	// 6. Build Response
	response := models.RelatedChangesResponse{
		AlertID:         alert.AlertId,
		RootEntityID:    rootEntityID,
		DirectChanges:   directChanges,
		NeighborChanges: neighborChanges,
	}

	if response.AlertID == "" {
		response.AlertID = alert.ID.Hex()
	}

	c.JSON(http.StatusOK, response)
}

// Helper to map DB Change to UI RelatedChange
func mapChange(ch models.Change, entityID string, hop int, scope string, alertStart time.Time) models.RelatedChange {
	overlap := "during_alert"
	if ch.StartTime.Before(alertStart) {
		overlap = "before_alert"
	}

	return models.RelatedChange{
		ChangeID:         ch.ChangeID,
		Name:             ch.Name,
		ChangeType:       ch.ChangeType,
		Status:           ch.Status,
		ImplementedBy:    ch.ImplementedBy,
		StartTime:        ch.StartTime,
		EndTime:          ch.EndTime,
		OverlapType:      overlap,
		ChangeScope:      scope,
		AffectedEntityID: entityID,
		HopDistance:      hop,
	}
}

func cloneMap(m bson.M) bson.M {
	newMap := bson.M{}
	for k, v := range m {
		newMap[k] = v
	}
	return newMap
}
