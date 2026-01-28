package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Change represents a change record from the ingestion service
type Change struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	ChangeID         string             `bson:"change_id"`
	Source           string             `bson:"source"`
	Name             string             `bson:"name"`
	Description      string             `bson:"description"`
	ChangeType       string             `bson:"change_type"`
	Status           string             `bson:"status"`
	ImplementedBy    string             `bson:"implemented_by"`
	AffectedEntities []string           `bson:"affected_entities"`
	StartTime        time.Time          `bson:"start_time"`
	EndTime          *time.Time         `bson:"end_time,omitempty"`
}

// RelatedChange is the UI-friendly representation with overlap details
type RelatedChange struct {
	ChangeID         string     `json:"change_id"`
	Name             string     `json:"name"`
	ChangeType       string     `json:"change_type"`
	Status           string     `json:"status"`
	ImplementedBy    string     `json:"implemented_by"`
	StartTime        time.Time  `json:"start_time"`
	EndTime          *time.Time `json:"end_time"`
	OverlapType      string     `json:"overlap_type"`
	
	// New fields for topology context
	ChangeScope      string     `json:"change_scope"`       // "direct" or "neighbor"
	AffectedEntityID string     `json:"affected_entity_id,omitempty"` // populated for neighbor changes
	HopDistance      int        `json:"hop_distance,omitempty"`       // 0 for direct, >0 for neighbor
}

// RelatedChangesResponse is the response payload for the API
type RelatedChangesResponse struct {
	AlertID         string          `json:"alert_id"`
	RootEntityID    string          `json:"root_entity_id"`
	DirectChanges   []RelatedChange `json:"direct_changes"`
	NeighborChanges []RelatedChange `json:"neighbor_changes"`
}
