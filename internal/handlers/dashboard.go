package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type DashboardStats struct {
	OpenIncidents   int64   `json:"open_incidents"`
	CriticalActive  int64   `json:"critical_active"`
	AverageMTTR     float64 `json:"average_mttr_minutes"`
	SystemHealth    float64 `json:"system_health"`
	EventsProcessed int64   `json:"events_processed_24h"`
}

type ServiceStat struct {
	Service  string `json:"service"`
	Critical int64  `json:"critical"`
	Warning  int64  `json:"warning"`
	Info     int64  `json:"info"`
	Total    int64  `json:"total"`
}

type TrendPoint struct {
	Timestamp string `json:"timestamp"`
	Count     int64  `json:"count"`
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int32:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	default:
		return 0
	}
}

func GetDashboardStats(c *gin.Context) {
	collection := db.GetCollection("alerts")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Open Incidents
	openFilter := bson.M{"alertstatus": "OPEN"}
	openCount, _ := collection.CountDocuments(ctx, openFilter)

	// 2. Critical Active (P0 or P1)
	criticalFilter := bson.M{
		"alertstatus":   "OPEN",
		"alertpriority": bson.M{"$in": bson.A{"P0", "P1"}},
	}
	criticalCount, _ := collection.CountDocuments(ctx, criticalFilter)

	// 3. Average MTTR (Last 7 days)
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	mttrPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"alertstatus":         "CLOSED",
			"alertcleartime.time": bson.M{"$gte": sevenDaysAgo},
		}}},
		{{Key: "$project", Value: bson.M{
			"duration": bson.M{"$divide": bson.A{
				bson.M{"$subtract": bson.A{"$alertcleartime.time", "$alertfirsttime.time"}},
				60000, // Convert to minutes
			}},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":     nil,
			"avgMTTR": bson.M{"$avg": "$duration"},
		}}},
	}

	var mttrResult []bson.M
	var avgMTTR float64
	cursor, err := collection.Aggregate(ctx, mttrPipeline)
	if err == nil {
		if cursor.All(ctx, &mttrResult); len(mttrResult) > 0 {
			if val, ok := mttrResult[0]["avgMTTR"].(float64); ok {
				avgMTTR = val
			}
		}
	}

	// 4. Events Processed (Last 24h)
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	eventCount, _ := collection.CountDocuments(ctx, bson.M{
		"alertfirsttime.time": bson.M{"$gte": oneDayAgo},
	})

	c.JSON(http.StatusOK, DashboardStats{
		OpenIncidents:   openCount,
		CriticalActive:  criticalCount,
		AverageMTTR:     avgMTTR,
		SystemHealth:    98.7,
		EventsProcessed: eventCount,
	})
}

func GetServiceHeatmap(c *gin.Context) {
	collection := db.GetCollection("alerts")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"alertstatus": "OPEN"}}},
		{{Key: "$group", Value: bson.M{
			"_id": "$servicename",
			"critical": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$in": bson.A{"$alertpriority", bson.A{"P0", "P1"}}}, 1, 0}}},
			"warning":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$alertpriority", "P2"}}, 1, 0}}},
			"info":     bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$in": bson.A{"$alertpriority", bson.A{"P3", "P4"}}}, 1, 0}}},
			"total":    bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"total": -1}}},
		{{Key: "$limit", Value: 10}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	heatmap := make([]ServiceStat, 0)
	for _, res := range results {
		// Use a safe check for ID type, sometimes it might be nil or empty string
		serviceName := "Unknown"
		if res["_id"] != nil {
			serviceName = res["_id"].(string)
		}

		heatmap = append(heatmap, ServiceStat{
			Service:  serviceName,
			Critical: toInt64(res["critical"]),
			Warning:  toInt64(res["warning"]),
			Info:     toInt64(res["info"]),
			Total:    toInt64(res["total"]),
		})
	}

	c.JSON(http.StatusOK, heatmap)
}

func GetAlertTrends(c *gin.Context) {
	collection := db.GetCollection("alerts")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oneDayAgo := time.Now().Add(-24 * time.Hour)
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"alertfirsttime.time": bson.M{"$gte": oneDayAgo}}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"$dateToString": bson.M{"format": "%H:00", "date": "$alertfirsttime.time"}, // Just hour for the trend
			},
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	trends := make([]TrendPoint, 0)
	for _, res := range results {
		trends = append(trends, TrendPoint{
			Timestamp: res["_id"].(string),
			Count:     toInt64(res["count"]),
		})
	}

	c.JSON(http.StatusOK, trends)
}
