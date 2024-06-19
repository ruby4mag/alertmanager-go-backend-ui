package handlers

import (
	"context"
	"fmt"
	"net/http"
    "time"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/gin-gonic/gin"
)



func New(c *gin.Context) {
    var alertRule  models.DbAlertRule
    collection := db.GetCollection("alertrules")

    if err := c.ShouldBindJSON(&alertRule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := collection.InsertOne(ctx, alertRule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result.InsertedID})
}


// Handler function to fetch all records
func Index(c *gin.Context) {
    collection := db.GetCollection("alertrules")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cur, err := collection.Find(ctx, bson.M{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer cur.Close(ctx)

    var records []bson.M
    for cur.Next(ctx) {
        var record bson.M
        if err := cur.Decode(&record); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        records = append(records, record)
    }
    if records == nil {
		records = []bson.M{}
	} 

    c.JSON(http.StatusOK, records)
}

// Handler function to get a record to edit.
func Edit(c *gin.Context) {

    id := c.Param("id")
    // Convert string ID to BSON ObjectID
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }
    collection := db.GetCollection("alertrules")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cur, err := collection.Find(ctx, bson.M{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer cur.Close(ctx)

    var record models.DbAlertRule
    err1 := collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&record)
    if err1 != nil {
        fmt.Println("Error is ", err1)
        c.JSON(http.StatusNotFound, gin.H{"message": "Item not found"})
        return
    }

    c.JSON(http.StatusOK, record)

}

// Handler function to update a record.
func Update(c *gin.Context) {
    var alertRule  models.DbAlertRule
    if err := c.ShouldBindJSON(&alertRule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

    id := c.Param("id")
    // Convert string ID to BSON ObjectID
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }

    collection := db.GetCollection("alertrules")
	updatefilter := bson.M{"_id": objectID }
    // Prepare the update document using the $set operator
	update := bson.M{"$set": alertRule}

    updateResult , updateerr := collection.UpdateOne(context.TODO(), updatefilter, update)
    if updateerr != nil {
        panic(updateerr)
    }
    if updateResult.ModifiedCount > 0 {
        fmt.Printf("Matched %v documents and updated %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
    }
	c.JSON(http.StatusOK, gin.H{"modified": updateResult.ModifiedCount})
}









