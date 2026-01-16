package models

import (
    "go.mongodb.org/mongo-driver/bson/primitive"
)

type DbCorrelationRule struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	GroupName       string             `bson:"groupname" json:"groupname"`
	Description     string             `bson:"description" json:"description"`
	GroupTags       []string           `bson:"grouptags" json:"grouptags"`
	GroupWindow     int                `bson:"groupwindow" json:"time_window_minutes"`
	CorrelationMode string             `bson:"correlation_mode" json:"correlation_mode"` // 'TAG_BASED' or 'SIMILARITY'
	ScopeTags       []string           `bson:"scope_tags" json:"scope_tags"`
	Similarity      SimilarityConfig   `bson:"similarity" json:"similarity"`
}

type SimilarityConfig struct {
	Fields    []string `bson:"fields" json:"fields"`
	Threshold float64  `bson:"threshold" json:"threshold"`
}
