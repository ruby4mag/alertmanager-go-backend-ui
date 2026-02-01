package handlers

import (
	"context"
	"time"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BuildRCAGraph constructs the RCA graph for a given alert
func BuildRCAGraph(ctx context.Context, alert models.DbAlert) (models.RCAGraphPayload, error) {
	rootEntityID := alert.Entity
	
	// 1. Build Graph Nodes & Edges
	nodes := []models.RCANode{}
	edges := []models.RCAEdge{}
	nodeMap := make(map[string]bool) // Track existing nodes to avoid dupes

	// Helper to add node safely
	addNode := func(id, typeStr string, attrs map[string]interface{}) {
		if !nodeMap[id] {
			nodes = append(nodes, models.RCANode{
				ID:         id,
				Type:       typeStr,
				Attributes: attrs,
			})
			nodeMap[id] = true
		}
	}

	// 1a. Add Alert Node
	alertNodeID := "alert:" + alert.AlertId
	if alert.AlertId == "" {
		alertNodeID = "alert:" + alert.ID.Hex()
	}
	addNode(alertNodeID, "alert", map[string]interface{}{
		"summary":    alert.AlertSummary,
		"severity":   alert.Severity,
		"start_time": alert.AlertFirstTime.Time,
		"status":     alert.AlertStatus,
	})

	// 1b. Add Change Context (Direct + Neighbors)
	// Discover Neighbors first
	neighborsMap := make(map[string]int) // entity -> distance
	neighborEntities := []string{}
	
	neo4jDriver := db.GetNeo4jDriver()
	if neo4jDriver != nil {
		session := neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		// Get neighbors and edges for Topology Context
		cypherQuery := `
		MATCH (root)
		WHERE root.name = $rootName OR root.id = $rootName
		MATCH p = (root)-[*1..6]-(neighbor)
		UNWIND relationships(p) as r
		WITH neighbor, r, length(p) as distance
		RETURN 
			startNode(r).name as source, 
			endNode(r).name as target, 
			type(r) as rel_type,
			neighbor.name as neighbor_name,
			distance
		LIMIT 1000
		`
		res, err := session.Run(ctx, cypherQuery, map[string]interface{}{"rootName": rootEntityID})
		if err == nil {
			for res.Next(ctx) {
				rec := res.Record()
				
				// Collect Neighbor info
				if nName, ok := rec.Get("neighbor_name"); ok {
					nameStr := nName.(string)
					distVal := int(rec.Values[4].(int64)) // Distance is index 4
					if _, exists := neighborsMap[nameStr]; !exists {
						neighborsMap[nameStr] = distVal
						neighborEntities = append(neighborEntities, nameStr)
						// Add Entity Node
						addNode("entity:"+nameStr, "entity", map[string]interface{}{"name": nameStr})
					} else {
                        if distVal < neighborsMap[nameStr] {
                            neighborsMap[nameStr] = distVal
                        }
                    }
				}

				// Collect Topology Edges
				src, _ := rec.Get("source")
				tgt, _ := rec.Get("target")
				relType, _ := rec.Get("rel_type")
				
				sStr := src.(string)
				tStr := tgt.(string)
				rType := relType.(string)

				// Ensure src/tgt nodes exist (might be root or intermediate)
				addNode("entity:"+sStr, "entity", map[string]interface{}{"name": sStr})
				addNode("entity:"+tStr, "entity", map[string]interface{}{"name": tStr})

				edges = append(edges, models.RCAEdge{
					From: "entity:" + sStr,
					To:   "entity:" + tStr,
					Type: rType, // e.g. "DEPENDS_ON"
				})
			}
		}
	}
	
	// Add Root Entity Node
	addNode("entity:"+rootEntityID, "entity", map[string]interface{}{
		"name": rootEntityID,
		"is_root": true,
	})
    // Add Alert -> Root Entity Edge
    edges = append(edges, models.RCAEdge{
        From: alertNodeID,
        To:   "entity:" + rootEntityID,
        Type: "AFFECTS_ENTITY",
    })

	// 1c. Fetch Changes
	effectiveEndTime := alert.AlertClearTime.Time
	if effectiveEndTime.IsZero() {
		effectiveEndTime = time.Now()
	}

	// Combine logical ORs:
    // (Time condition) AND (Entity condition)
    // Entity condition is: affected_entities == root OR affected_entities IN neighbors
    
    entityCondition := bson.A{
        bson.M{"affected_entities": rootEntityID},
    }
    if len(neighborEntities) > 0 {
        entityCondition = append(entityCondition, bson.M{"affected_entities": bson.M{"$in": neighborEntities}})
    }

    // Final Filter construction
    finalFilter := bson.M{
        "$and": []bson.M{
            {"status": bson.M{"$in": []string{"scheduled", "in_progress", "completed"}}},
            {"start_time": bson.M{"$lte": effectiveEndTime}},
            {"$or": []bson.M{
                {"end_time": nil},
                {"end_time": bson.M{"$exists": false}},
                {"end_time": bson.M{"$gte": alert.AlertFirstTime.Time}},
            }},
            {"$or": entityCondition},
        },
    }

	changesCollection := db.GetCollection("changes")
	cursor, err := changesCollection.Find(ctx, finalFilter, options.Find().SetLimit(100)) // Cap changes
	if err == nil {
		var changes []models.Change
		if err := cursor.All(ctx, &changes); err == nil {
			for _, ch := range changes {
				// Determine Scope & Entity Connection
				scope := "neighbor"
				hop := 0
				targetEntity := "" 
				
                // Check if direct
				isDirect := false
				for _, aff := range ch.AffectedEntities {
					if aff == rootEntityID {
						isDirect = true
                        targetEntity = rootEntityID
						break
					}
				}
				
				if isDirect {
					scope = "direct"
					hop = 0
				} else {
					// Find closest neighbor
					minDist := 100
					for _, aff := range ch.AffectedEntities {
						if d, ok := neighborsMap[aff]; ok {
							if d < minDist {
								minDist = d
								targetEntity = aff
							}
						}
					}
					hop = minDist
                    if targetEntity == "" {
                         // Should not happen given query, but strictly safe
                        continue 
                    }
				}

				// Create Change Node
				changeNodeID := "change:" + ch.ChangeID
				
				attrs := map[string]interface{}{
					"change_id":      ch.ChangeID,
					"name":           ch.Name,
					"change_type":    ch.ChangeType,
					"status":         ch.Status,
					"implemented_by": ch.ImplementedBy,
					"start_time":     ch.StartTime,
					"end_time":       ch.EndTime,
					"scope":          scope,
					"hop_distance":   hop,
				}
				addNode(changeNodeID, "change", attrs)

				// Edge: Change -> Entity
				edges = append(edges, models.RCAEdge{
					From: changeNodeID,
					To:   "entity:" + targetEntity,
					Type: "AFFECTS",
				})

				// Edge: Change -> Alert (Temporal)
				overlap := "during_alert"
				if ch.StartTime.Before(alert.AlertFirstTime.Time) {
					overlap = "before_alert"
				}
				
				edges = append(edges, models.RCAEdge{
					From: changeNodeID,
					To:   alertNodeID,
					Type: "TEMPORAL_OVERLAP",
					Attributes: map[string]interface{}{
						"overlap_type": overlap,
					},
				})
			}
		}
	}

	// 2. Construct Payload
	payload := models.RCAGraphPayload{
		RCAContext: models.RCAContext{
			AlertID:      alert.AlertId,
			RootEntityID: rootEntityID,
			SessionID:    alert.AlertId, // Use incident ID as sessionId for Redis context in n8n
			GeneratedAt:  time.Now(),
		},
		Nodes: nodes,
		Edges: edges,
	}
	
	return payload, nil
}
