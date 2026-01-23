package models

import (
    "time"
)

// AIRCA represents the immutable AI analysis of an incident
type AIRCA struct {
    IncidentID  string    `bson:"incident_id" json:"incident_id"`
    AIRootCause string    `bson:"ai_root_cause" json:"ai_root_cause"`
    AIConfidence float64  `bson:"ai_confidence" json:"ai_confidence"`
    AIReasoning string    `bson:"ai_reasoning" json:"ai_reasoning"`
    CreatedAt   time.Time `bson:"created_at" json:"created_at"`
}

// IncidentFeedback represents the authoritative human feedback
type IncidentFeedback struct {
    FeedbackID        string    `bson:"feedback_id" json:"feedback_id"`
    IncidentID        string    `bson:"incident_id" json:"incident_id"`
    Verdict           string    `bson:"verdict" json:"verdict"` // correct | incorrect | partial
    FinalRootCause    string    `bson:"final_root_cause" json:"final_root_cause"` // Entity ID
    RootCauseType     string    `bson:"root_cause_type" json:"root_cause_type"` // infrastructure | application | network | external
    Symptoms          []string  `bson:"symptoms" json:"symptoms"` // List of entity IDs
    ResolutionSummary string    `bson:"resolution_summary" json:"resolution_summary"`
    WhyAIWasWrong     []string  `bson:"why_ai_was_wrong" json:"why_ai_was_wrong"`
    OperatorConfidence float64  `bson:"operator_confidence" json:"operator_confidence"` 
    SubmittedBy       string    `bson:"submitted_by" json:"submitted_by"`
    SubmittedAt       time.Time `bson:"submitted_at" json:"submitted_at"`
}

// RCACaseMemory serves as the normalized source for RAG
type RCACaseMemory struct {
    CaseID            string    `bson:"case_id" json:"case_id"`
    IncidentID        string    `bson:"incident_id" json:"incident_id"`
    AlertSignature    string    `bson:"alert_signature" json:"alert_signature"`
    TopologySignature string    `bson:"topology_signature" json:"topology_signature"`
    TemporalSignature string    `bson:"temporal_signature" json:"temporal_signature"`
    RootCauseEntity   string    `bson:"root_cause_entity" json:"root_cause_entity"`
    RootCauseType     string    `bson:"root_cause_type" json:"root_cause_type"`
    OwningTeam        string    `bson:"owning_team" json:"owning_team"`
    ResolutionSummary string    `bson:"resolution_summary" json:"resolution_summary"`
    Confidence        float64   `bson:"confidence" json:"confidence"`
    CreatedAt         time.Time `bson:"created_at" json:"created_at"`
}
