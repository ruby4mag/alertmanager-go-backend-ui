package models

import "time"

// RCAGraphPayload represents the full graph payload sent to n8n for RCA
type RCAGraphPayload struct {
	RCAContext RCAContext `json:"rca_context"`
	Nodes      []RCANode  `json:"nodes"`
	Edges      []RCAEdge  `json:"edges"`
}

type RCAContext struct {
	AlertID      string    `json:"alert_id"`
	RootEntityID string    `json:"root_entity_id"`
	GeneratedAt  time.Time `json:"generated_at"`
}

type RCANode struct {
	ID         string                 `json:"id"`         // e.g. "change:chg-1", "entity:db-1", "alert:a-1"
	Type       string                 `json:"type"`       // "change", "entity", "alert"
	Attributes map[string]interface{} `json:"attributes"` // Flexible attributes
}

type RCAEdge struct {
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Type       string                 `json:"type"`       // "AFFECTS", "TEMPORAL_OVERLAP", "CONNECTED_TO"
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}
