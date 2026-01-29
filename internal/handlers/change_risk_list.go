package handlers

import (
    "context"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
    "github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo/options"
)

// ListChangesWithRisk retrieves all changes and computes/returns their risk details
func ListChangesWithRisk(c *gin.Context) {
    // 1. Fetch Changes from DB
    col := db.GetCollection("changes")
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Example: Fetch latest 50 changes
    findOptions := options.Find()
    findOptions.SetSort(bson.D{{Key: "start_time", Value: -1}})
    findOptions.SetLimit(50)

    cursor, err := col.Find(ctx, bson.M{}, findOptions)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch changes"})
        return
    }
    defer cursor.Close(ctx)

    var changes []models.Change
    if err := cursor.All(ctx, &changes); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode changes"})
        return
    }

    // 2. Compute Risk for each change (or fetch if pre-calculated)
    // For this requirement, we calculate on the fly or construct a view model.
    // Since we don't store risk scores yet, we'll calculate them here.

    type ChangeRiskView struct {
        ChangeID       string               `json:"change_id"`
        Name           string               `json:"name"`
        Description    string               `json:"description"` // New
        ChangeType     string               `json:"change_type"`
        Status         string               `json:"status"`
        Source         string               `json:"source"`      // New
        ImplementedBy  string               `json:"implemented_by"` // New
        AffectedEntities []string           `json:"affected_entities"` // New
        StartTime      time.Time            `json:"start_time"`
        EndTime        *time.Time           `json:"end_time"`    // New
        Risk           models.RiskResult    `json:"risk_details"`
    }

    results := []ChangeRiskView{}

    for _, ch := range changes {
        // Construct mock/real topology facts for the risk calculator
        mockFacts := models.TopologyFacts{
             DirectDependentsCount: 2, 
             IndirectDependentsCount: 5,
             NodeTier: "Tier-1",
             NeighborTiers: []string{"Tier-2"},
             ConcurrentChanges: 0,
             HasRollbackPlan: true, 
        }
        
        // Use the existing logic (refactored for internal use) or inline logical
        riskInput := models.RiskInput{
            Change: models.ChangeDetails{
                ChangeID: ch.ChangeID,
                Node: "unknown", // ch.AffectedEntities[0] if exists
                ChangeType: ch.ChangeType,
                ChangeScope: "direct",
            },
            TopologyFacts: mockFacts,
        }
        
        // Calculate Risk (Internal Function Call)
        riskResult := computeRiskInternal(riskInput)

        results = append(results, ChangeRiskView{
            ChangeID:      ch.ChangeID,
            Name:          ch.Name,
            Description:   ch.Description,
            ChangeType:    ch.ChangeType,
            Status:        ch.Status,
            Source:        ch.Source,
            ImplementedBy: ch.ImplementedBy,
            AffectedEntities: ch.AffectedEntities,
            StartTime:     ch.StartTime,
            EndTime:       ch.EndTime,
            Risk:          riskResult,
        })
    }

    c.JSON(http.StatusOK, results)
}

// Internal helper to reuse logic without HTTP context
func computeRiskInternal(input models.RiskInput) models.RiskResult {
    breakdown := models.RiskBreakdown{}
    
    // Logic from CalculateChangeRisk (duplicated for now to separate concern or should be shared service)
    radiusScore := (input.TopologyFacts.DirectDependentsCount * 2) + (input.TopologyFacts.IndirectDependentsCount * 1)
    if radiusScore > 30 { radiusScore = 30 }
    breakdown.BlastRadius = radiusScore

    switch input.TopologyFacts.NodeTier {
    case "Tier-0": breakdown.NodeTier = 30
    case "Tier-1": breakdown.NodeTier = 20
    case "Tier-2": breakdown.NodeTier = 10
    default: breakdown.NodeTier = 5
    }

    breakdown.NeighborTier = 10 // Mock default
    
    switch input.Change.ChangeType {
    case "db": breakdown.ChangeType = 10
    case "config": breakdown.ChangeType = 8
    case "code": breakdown.ChangeType = 5
    default: breakdown.ChangeType = 2
    }
    
    breakdown.ChangeScope = 10

    breakdown.Modifiers = 0

    totalScore := breakdown.BlastRadius + breakdown.NodeTier + breakdown.NeighborTier + breakdown.ChangeType + breakdown.ChangeScope + breakdown.Modifiers
    if totalScore > 100 { totalScore = 100 }

    level := "LOW"
    if totalScore >= 80 {
        level = "CRITICAL"
    } else if totalScore >= 50 {
        level = "HIGH"
    } else if totalScore >= 20 {
        level = "MEDIUM"
    }

    return models.RiskResult{
        ChangeID: input.Change.ChangeID,
        RiskScore: totalScore,
        RiskLevel: level,
        RiskBreakdown: breakdown,
        ComputedAt: time.Now(),
    }
}
