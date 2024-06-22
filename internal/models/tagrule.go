package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DbTagRule struct {
	ID 					primitive.ObjectID 	`bson:"_id,omitempty"`
	RuleName			string 				`bson:"rulename" json:"rulename"`
	RuleDescription 	string 				`bson:"ruledescription" json:"ruledescription"`
	RuleObject			string  			`bson:"ruleobject" json:"ruleobject"`
	Order				int  				`bson:"order" json:"order"`
	FieldName			string				`bson:"fieldname" json:"fieldname"`
	TagName				string 				`bson:"tagname" json:"tagname"`
	FieldExtraction		string				`bson:"fieldextraction" json:"fieldextraction"`
	TagValue			string 				`bson:"tagvalue" json:"tagvalue"`
}

