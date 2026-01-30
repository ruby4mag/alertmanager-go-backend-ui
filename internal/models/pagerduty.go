package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DbPagerDutyService represents a PagerDuty service stored in MongoDB
type DbPagerDutyService struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ServiceID   string             `bson:"service_id" json:"service_id"`
	ServiceName string             `bson:"service_name" json:"service_name"`
}

// DbPagerDutyEscalationPolicy represents a PagerDuty escalation policy stored in MongoDB
type DbPagerDutyEscalationPolicy struct {
	ID     primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	EpID   string             `bson:"ep_id" json:"ep_id"`
	EpName string             `bson:"ep_name" json:"ep_name"`
}

// PagerDutyServiceResponse is the response format for the API
type PagerDutyServiceResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// PagerDutyEscalationPolicyResponse is the response format for the API
type PagerDutyEscalationPolicyResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
