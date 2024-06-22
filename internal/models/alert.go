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
// 
type DbAlert struct {
	ID 				primitive.ObjectID `bson:"_id,omitempty"`
	Entity			string 				`json:"entity"`
	AlertFirstTime	CustomTime 			`json:"alertfirsttime"`
	AlertLastTime	CustomTime  		`json:"alertlasttime"`
	AlertClearTime	CustomTime  		`json:"alertcleartime"`
	AlertSource		string 				`json:"alertsource"`
	ServiceName 	string 				`json:"servicename"`
	AlertSummary	string 				`json:"alertsummary"`
	AlertStatus		string 				`json:"alertstatus"`
	AlertNotes		string 				`json:"alertnotes"`
	AlertAcked		string 				`json:"alertacked"`
	Severity		string 				`json:"severity"`
	AlertId			string				`json:"alertid"`
	AlertPriority	string				`json:"alertpriority"`
	IpAddress		string				`json:"ipaddress"`
	AlertType		string				`json:"alerttype"`
	AlertCount		int					`json:"alertcount"`
	AlertDropped	string				`json:"alertdropped"`
	AdditionalDetails	map[string]interface{}			`json:"additionaldetails"`
	WorkLogs		[]WorkLog			`json:"worklogs"`
	GroupIdentifier	string				`json:"groupidentifier"`
	Grouped 		bool				`json:"grouped"`
	GroupIncidentId	string				`json:"groupincidentid"`	
	GroupAlerts		[]primitive.ObjectID			`json:"groupalerts"`
	Parent			bool				`json:"parent"`
}

type WorkLog struct {
    ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
    Author    string             `bson:"author" json:"author"`
    Comment   string             `bson:"comment" json:"comment"`
    CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

