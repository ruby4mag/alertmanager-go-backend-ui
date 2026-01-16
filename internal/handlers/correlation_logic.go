package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CorrelateAlert is the main entry point to process an alert against active rules.
// It should be called after an alert is ingested or updated.
func CorrelateAlert(ctx context.Context, alert models.DbAlert) error {
	// 1. Fetch all active Correlation Rules
	rulesCol := db.GetCollection("correlationrules")
	cursor, err := rulesCol.Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	var rules []models.DbCorrelationRule
	if err := cursor.All(ctx, &rules); err != nil {
		return err
	}

	alertsCol := db.GetCollection("alerts")

	for _, rule := range rules {
		// Skip if rule has no window defined (safety)
		if rule.GroupWindow <= 0 {
			continue
		}

		// Calculate time window
		cutoff := time.Now().Add(time.Duration(-rule.GroupWindow) * time.Minute)

		// 2. Find Candidates: Active alerts (not cleared) within time window
		// We look for alerts that are NOT the current alert
		filter := bson.M{
			"_id":             bson.M{"$ne": alert.ID},
			"alertstatus":     bson.M{"$ne": "CLOSED"}, // Only correlate open alerts
			"alertfirsttime":  bson.M{"$gte": cutoff},  // Within window
            "grouped":         false,                   // Only look for ungrouped alerts? Or parents? 
                                                        // Complex topic: usually we look for open Groups (parents) first.
		}

        // Logic split based on Mode
		if rule.CorrelationMode == "SIMILARITY" {
			matched, reason, score := findSimilarityMatch(ctx, alert, rule, alertsCol, filter)
			if matched != nil {
				return groupAlerts(ctx, alertsCol, *matched, alert, rule, reason, score)
			}
		} else {
            // Default TAG_BASED (Simple implementation hook)
            // Implementation omitted as per user request focus on SIMILARITY, 
            // but structure is here for backward compatibility.
            // matched := findTagMatch(ctx, alert, rule, alertsCol, filter)
		}
	}
	return nil
}

// findSimilarityMatch searches for a candidate alert/group that matches the similarity rule.
func findSimilarityMatch(ctx context.Context, sourceAlert models.DbAlert, rule models.DbCorrelationRule, col *mongo.Collection, baseFilter bson.M) (*models.DbAlert, *models.GroupingReason, float64) {
	
    // 1. Scope Filtering (Hard Constraint)
    // We refine the filter to enforce scope tags
    scopeFilter := baseFilter
    reasons := []string{}

    for _, tag := range rule.ScopeTags {
        val := getFieldOrTag(sourceAlert, tag)
        if val == "" {
            // Source alert missing required scope tag -> cannot match this rule
            return nil, nil, 0
        }
        // Enforce candidate has same value
        // Note: Logic allows checking struct fields or AdditionalDetails
        if isStructField(tag) {
             scopeFilter[strings.ToLower(tag)] = val
        } else {
             scopeFilter["additionaldetails."+tag] = val
        }
        reasons = append(reasons, fmt.Sprintf("Same %s: %s", tag, val))
    }

    // Fetch candidates passing scope
    // Optimization: limit candidates?
    candidatesCur, err := col.Find(ctx, scopeFilter, options.Find().SetLimit(50))
    if err != nil {
        return nil, nil, 0
    }
    defer candidatesCur.Close(ctx)

    var bestMatch *models.DbAlert
    var bestScore float64 = 0

    // 2. Similarity Calculation (Soft Constraint)
    for candidatesCur.Next(ctx) {
        var candidate models.DbAlert
        if err := candidatesCur.Decode(&candidate); err != nil {
            continue
        }

        score := calculateSimilarity(sourceAlert, candidate, rule.Similarity.Fields)
        if score >= rule.Similarity.Threshold && score > bestScore {
            bestScore = score
            matchCopy := candidate
            bestMatch = &matchCopy
        }
    }

    if bestMatch != nil {
        reasons = append(reasons, fmt.Sprintf("Similar content (Score: %.2f)", bestScore))
        reasonObj := &models.GroupingReason{
            Type:        "SIMILARITY",
            Description: "Grouped by similarity rule: " + rule.GroupName,
            Reasons:     reasons,
        }
        return bestMatch, reasonObj, bestScore
    }

    return nil, nil, 0
}

func calculateSimilarity(a, b models.DbAlert, fields []string) float64 {
    var textA, textB string
    for _, f := range fields {
        textA += " " + getFieldOrTag(a, f)
        textB += " " + getFieldOrTag(b, f)
    }
    return jaccardSimilarity(textA, textB)
}

func jaccardSimilarity(s1, s2 string) float64 {
    tokens1 := tokenize(s1)
    tokens2 := tokenize(s2)

    if len(tokens1) == 0 && len(tokens2) == 0 {
        return 0
    }

    intersection := 0
    seen := make(map[string]bool)
    for _, t := range tokens1 {
        seen[t] = true
    }
    for _, t := range tokens2 {
        if seen[t] {
            intersection++
            // Avoid double counting if using set logic? 
            // Jaccard is |A ^ B| / |A v B|.
            // Using boolean map for set A.
        }
    }
    
    // Correct Jaccard Set Implementation
    setA := make(map[string]struct{})
    for _, t := range tokens1 { setA[t] = struct{}{} }
    setB := make(map[string]struct{})
    for _, t := range tokens2 { setB[t] = struct{}{} }
    
    inter := 0
    for k := range setA {
        if _, ok := setB[k]; ok {
            inter++
        }
    }
    
    union := len(setA) + len(setB) - inter
    if union == 0 { return 0 }
    return float64(inter) / float64(union)
}

func tokenize(s string) []string {
    s = strings.ToLower(s)
    return strings.Fields(s)
}

func getFieldOrTag(alert models.DbAlert, key string) string {
    // Check known struct fields first (case insensitive mapping)
    switch strings.ToLower(key) {
    case "summary", "alertsummary": return alert.AlertSummary
    case "servicename", "service": return alert.ServiceName
    case "entity": return alert.Entity
    case "source", "alertsource": return alert.AlertSource
    case "severity": return alert.Severity
    case "notes", "alertnotes": return alert.AlertNotes
    // Add more mappings as needed
    }
    // Fallback to AdditionalDetails
    if v, ok := alert.AdditionalDetails[key]; ok {
        return fmt.Sprintf("%v", v)
    }
    return ""
}

func isStructField(key string) bool {
    switch strings.ToLower(key) {
    case "summary", "alertsummary", "servicename", "service", "entity", "source", "alertsource", "severity", "notes", "alertnotes":
        return true
    }
    return false
}

// groupAlerts handles creating a parent or merging into existing parent
func groupAlerts(ctx context.Context, col *mongo.Collection, match models.DbAlert, current models.DbAlert, rule models.DbCorrelationRule, reason *models.GroupingReason, score float64) error {
    
    // Scenario A: Match is already a Parent
    if match.Parent {
        // Add current to match's children
        // Add current ID to GroupAlerts
        // Update current to point to match
        
        // 1. Update Parent
        _, err := col.UpdateOne(ctx, bson.M{"_id": match.ID}, bson.M{
            "$addToSet": bson.M{"groupalerts": current.ID},
            "$set": bson.M{"grouping_reason": reason}, // Update reason? Or merge? Usually we keep initial reason or append.
        })
        if err != nil { return err }

        // 2. Update Child (Current)
        _, err = col.UpdateOne(ctx, bson.M{"_id": current.ID}, bson.M{
            "$set": bson.M{
                "grouped": true,
                "groupincidentid": match.AlertId, // Assuming AlertID is used as display ID
                "parent": false,
            },
        })
        return err
    }

    // Scenario B: Match is a Child (Already grouped) -> We should normally find the Parent instead.
    if match.Grouped && !match.Parent {
        // Find parent
        var parent models.DbAlert
        // Assuming GroupIncidentId links to Parent's AlertId? Or we search parent having this child?
        // Logic: Find Alert where GroupAlerts contains match.ID
        err := col.FindOne(ctx, bson.M{"groupalerts": match.ID}).Decode(&parent)
        if err == nil {
             // Pass to parent logic
             return groupAlerts(ctx, col, parent, current, rule, reason, score)
        }
    }

    // Scenario C: Match is a standalone alert. Create a NEW Parent.
    // Create Parent Alert
    parentID := primitive.NewObjectID()
    parentAlert := models.DbAlert{
        ID: parentID,
        AlertId: fmt.Sprintf("GRP-%d", time.Now().Unix()),
        AlertSummary: fmt.Sprintf("Group: %s (%s)", rule.GroupName, current.AlertSummary),
        Entity: "Multiple",
        Severity: match.Severity, // Inherit or calc max
        Parent: true,
        Grouped: true,
        GroupAlerts: []primitive.ObjectID{match.ID, current.ID},
        GroupingReason: reason,
        AlertStatus: "OPEN",
        AlertFirstTime: match.AlertFirstTime, // Earliest
        AlertLastTime: current.AlertLastTime, // Latest
    }

    _, err := col.InsertOne(ctx, parentAlert)
    if err != nil { return err }

    // Update both children
    updateChild := bson.M{
        "$set": bson.M{
            "grouped": true,
            "groupincidentid": parentAlert.AlertId,
            "parent": false,
        },
    }
    _, err = col.UpdateOne(ctx, bson.M{"_id": match.ID}, updateChild)
    _, err = col.UpdateOne(ctx, bson.M{"_id": current.ID}, updateChild)

    return err
}
