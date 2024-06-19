package models

import (
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CustomTime struct {
    time.Time
}

func (ct *CustomTime) UnmarshalJSON(b []byte) error {
    s := strings.Trim(string(b), `"`) // Remove quotes from the string
	if len(b) < 3 {
		ct.Time = time.Time{}
		return nil
	}
    t, err := time.Parse("2006-01-02 15:04:05", s)
    if err != nil {
        return err
    }
    ct.Time = t
    return nil
}

type DbAlert struct {
	ID 				primitive.ObjectID `bson:"_id,omitempty"`
	Entity			string 				`json:"entity"`
	AlertFirstTime	CustomTime 			`json:"alertTime"`
	AlertLastTime	CustomTime  		`json:"alertLastTime"`
	AlertClearTime	CustomTime  		`json:"alertClearTime"`
	AlertSource		string 				`json:"alertSource"`
	ServiceName 	string 				`json:"serviceName"`
	AlertSummary	string 				`json:"alertSummary"`
	AlertStatus		string 				`json:"alertStatus"`
	AlertNotes		string 				`json:"alertNotes"`
	AlertAcked		string 				`json:"alertAcked"`
	Severity		string 				`json:"severity"`
	AlertId			string				`json:"alertId"`
	AlertPriority	string				`json:"alertPriority"`
	IpAddress		string				`json:"ipAddress"`
	AlertType		string				`json:"alertType"`
	AlertCount		int					`json:"alertCount"`
	AlertDropped	string				`json:"alertDropped"`
	AdditionalDetails	map[string]interface{}			`json:"additionalDetails"`
	WorkLogs			[]WorkLog			`bson:"worklogs" json:"worklogs`
}

type WorkLog struct {
    ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
    Author    string             `bson:"author" json:"author"`
    Comment   string             `bson:"comment" json:"comment"`
    CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

