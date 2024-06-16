package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DbAlertRule struct {
	ID 					primitive.ObjectID `bson:"_id,omitempty"`
	RuleName			string 				`bson:"rulename" json:"rulename"`
	RuleDescription 	string 				`bson:"ruledescription" json:"ruledescription"`
	RuleObject			string  			`bson:"ruleobject" json:"ruleobject"`
	Order				int  				`bson:"order" json:"order"`
	SetField			string				`bson:"setfield" json:"setfield"`
	SetValue			string				`bson:"setvalue" json:"setvalue"`
}