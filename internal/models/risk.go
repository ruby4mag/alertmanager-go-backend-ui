package models

import "time"

// RiskInput represents the payload for calculating risk
type RiskInput struct {
	Change        ChangeDetails `json:"change"`
	TopologyFacts TopologyFacts `json:"topology_facts"`
}

type ChangeDetails struct {
	ChangeID    string `json:"change_id"`
	Node        string `json:"node"`
	ChangeType  string `json:"change_type"`
	ChangeScope string `json:"change_scope"` // "direct" | "indirect"
}

type TopologyFacts struct {
	DirectDependentsCount   int      `json:"direct_dependents_count"`
	IndirectDependentsCount int      `json:"indirect_dependents_count"`
	NodeTier                string   `json:"node_tier"` // "Tier-0", "Tier-1", etc.
	NeighborTiers           []string `json:"neighbor_tiers"`
	ConcurrentChanges       int      `json:"concurrent_changes"`
	HasRollbackPlan         bool     `json:"has_rollback_plan"`
}

// RiskResult represents the computed risk score
type RiskResult struct {
	ChangeID      string        `json:"change_id"`
	RiskScore     int           `json:"risk_score"`
	RiskLevel     string        `json:"risk_level"`
	RiskBreakdown RiskBreakdown `json:"risk_breakdown"`
	ComputedAt    time.Time     `json:"computed_at"`
}

type RiskBreakdown struct {
	BlastRadius  int `json:"blast_radius"`
	NodeTier     int `json:"node_tier"`
	NeighborTier int `json:"neighbor_tier"` // Max impact from neighbor
	ChangeType   int `json:"change_type"`
	ChangeScope  int `json:"change_scope"`
	Modifiers    int `json:"modifiers"`
}
