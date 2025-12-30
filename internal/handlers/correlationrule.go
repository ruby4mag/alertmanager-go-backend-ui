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

func NewCorrelation(c *gin.Context) {
    var rule models.DbCorrelationRule
    collection := db.GetCollection("correlationrules")

    if err := c.ShouldBindJSON(&rule); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    result, err := collection.InsertOne(ctx, rule)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"result": result.InsertedID})
}

// Handler function to fetch all correlation rules
func IndexCorrelation(c *gin.Context) {
    collection := db.GetCollection("correlationrules")
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

// Handler function to get a single correlation rule by id
func EditCorrelation(c *gin.Context) {
    id := c.Param("id")
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }
    collection := db.GetCollection("correlationrules")

    var record models.DbCorrelationRule
    err1 := collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&record)
    if err1 != nil {
        fmt.Println("Error is ", err1)
        c.JSON(http.StatusNotFound, gin.H{"message": "Item not found"})
        return
    }

    c.JSON(http.StatusOK, record)
}

// Handler function to update a correlation rule
func UpdateCorrelation(c *gin.Context) {
    var rule models.DbCorrelationRule
    if err := c.ShouldBindJSON(&rule); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    id := c.Param("id")
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }

    collection := db.GetCollection("correlationrules")
    updatefilter := bson.M{"_id": objectID}
    update := bson.M{"$set": rule}

    updateResult, updateerr := collection.UpdateOne(context.TODO(), updatefilter, update)
    if updateerr != nil {
        panic(updateerr)
    }
    if updateResult.ModifiedCount > 0 {
        fmt.Printf("Matched %v documents and updated %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
    }
    c.JSON(http.StatusOK, gin.H{"modified": updateResult.ModifiedCount})
}
