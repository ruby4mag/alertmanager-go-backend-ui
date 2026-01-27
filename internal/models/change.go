package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Change represents a change record from the ingestion service
type Change struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	ChangeID         string             `bson:"change_id"`
	Name             string             `bson:"name"`
	ChangeType       string             `bson:"change_type"`
	Status           string             `bson:"status"`
	ImplementedBy    string             `bson:"implemented_by"`
	AffectedEntities []string           `bson:"affected_entities"`
	StartTime        time.Time          `bson:"start_time"`
	EndTime          *time.Time         `bson:"end_time,omitempty"`
}

// RelatedChange is the UI-friendly representation with overlap details
type RelatedChange struct {
	ChangeID      string     `json:"change_id"`
	Name          string     `json:"name"`
	ChangeType    string     `json:"change_type"`
	Status        string     `json:"status"`
	ImplementedBy string     `json:"implemented_by"`
	StartTime     time.Time  `json:"start_time"`
	EndTime       *time.Time `json:"end_time"`
	OverlapType   string     `json:"overlap_type"`
}

// RelatedChangesResponse is the response payload for the API
type RelatedChangesResponse struct {
	AlertID        string          `json:"alert_id"`
	EntityID       string          `json:"entity_id"`
	RelatedChanges []RelatedChange `json:"related_changes"`
}
