package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
)

// CalculateChangeRisk computes a deterministic risk score based on inputs
func CalculateChangeRisk(c *gin.Context) {
	var input models.RiskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	breakdown := models.RiskBreakdown{}
	
	// 1. Blast Radius Score (Max 30)
	// Direct deps: 2 pts each, Indirect: 1 pt each
	radiusScore := (input.TopologyFacts.DirectDependentsCount * 2) + (input.TopologyFacts.IndirectDependentsCount * 1)
	if radiusScore > 30 {
		radiusScore = 30
	}
	breakdown.BlastRadius = radiusScore

	// 2. Node Tier Score (Max 30)
	switch input.TopologyFacts.NodeTier {
	case "Tier-0":
		breakdown.NodeTier = 30
	case "Tier-1":
		breakdown.NodeTier = 20
	case "Tier-2":
		breakdown.NodeTier = 10
	case "Tier-3":
		breakdown.NodeTier = 5
	default:
		breakdown.NodeTier = 0
	}

	// 3. Neighbor Tier Impact (Max 20)
	// If a critical neighbor is affected, risk increases
	maxNeighborScore := 0
	for _, tier := range input.TopologyFacts.NeighborTiers {
		score := 0
		switch tier {
		case "Tier-0":
			score = 20
		case "Tier-1":
			score = 10
		}
		if score > maxNeighborScore {
			maxNeighborScore = score
		}
	}
	breakdown.NeighborTier = maxNeighborScore
	
	// 4. Change Type (Max 10)
	switch input.Change.ChangeType {
	case "db":
		breakdown.ChangeType = 10
	case "config":
		breakdown.ChangeType = 8
	case "code":
		breakdown.ChangeType = 5
	case "deployment":
		breakdown.ChangeType = 5
	default:
		breakdown.ChangeType = 2
	}
	
	// 5. Change Scope (Max 10)
	if input.Change.ChangeScope == "direct" {
		breakdown.ChangeScope = 10
	} else if input.Change.ChangeScope == "indirect" {
        breakdown.ChangeScope = 5
    }

	// 6. Modifiers
	// Concurrent Changes: +5 each (Max 15)
	concurrentPenalty := input.TopologyFacts.ConcurrentChanges * 5
	if concurrentPenalty > 15 {
		concurrentPenalty = 15
	}
	
	// No Rollback Plan: +20 Penalty
	rollbackPenalty := 0
	if !input.TopologyFacts.HasRollbackPlan {
		rollbackPenalty = 20
	}
	
	breakdown.Modifiers = concurrentPenalty + rollbackPenalty

	// Calculate Total
	totalScore := breakdown.BlastRadius + 
		breakdown.NodeTier + 
		breakdown.NeighborTier + 
		breakdown.ChangeType + 
		breakdown.ChangeScope + 
		breakdown.Modifiers

	if totalScore > 100 {
		totalScore = 100
	}

	// Determine Risk Level
	level := "LOW"
	if totalScore >= 80 {
		level = "CRITICAL"
	} else if totalScore >= 50 {
		level = "HIGH"
	} else if totalScore >= 20 {
		level = "MEDIUM"
	}

	result := models.RiskResult{
		ChangeID:      input.Change.ChangeID,
		RiskScore:     totalScore,
		RiskLevel:     level,
		RiskBreakdown: breakdown,
		ComputedAt:    time.Now(),
	}

	c.JSON(http.StatusOK, result)
}
