package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func Alerts(c *gin.Context) {

	type Filter struct {
		Id    string `json:"id"`
		Value string `json:"value"`
	}
	
	type Sorting struct {
		Id   string `json:"id"`
		Desc bool   `json:"desc"`
	}
	start, _ := strconv.Atoi(c.DefaultQuery("start", "0"))
    size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
    globalFilter := c.Query("globalFilter")
    sortQuery := c.Query("sorting")

	var filters []Filter
    _ = json.Unmarshal([]byte(c.Query("filters")), &filters)
    
    var sorting []Sorting
    _ = json.Unmarshal([]byte(sortQuery), &sorting)

    filter := bson.M{"grouped" : false}
    // filter["grouped"] = bson.M{"grouped" : true}
    if globalFilter != "" {
        filter["$or"] = []bson.M{
            {"name": bson.M{"$regex": globalFilter, "$options": "i"}},
            {"email": bson.M{"$regex": globalFilter, "$options": "i"}},
        }
    }
    for _, f := range filters {
        filter[f.Id] = bson.M{"$regex": f.Value, "$options": "i"}
    }

    findOptions := options.Find()
    findOptions.SetSkip(int64(start))
    findOptions.SetLimit(int64(size))

    if len(sorting) > 0 {
        sortFields := bson.D{}
        for _, s := range sorting {
            sortOrder := 1
            if s.Desc {
                sortOrder = -1
            }
            sortFields = append(sortFields, bson.E{Key: s.Id, Value: sortOrder})
        }
        findOptions.SetSort(sortFields)
    }
	collection := db.GetCollection("alerts")

	cursor, err := collection.Find(ctx, filter, findOptions)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    var alerts []models.DbAlert
    if err := cursor.All(ctx, &alerts); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    count, err := collection.CountDocuments(ctx, filter)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

	if len(alerts) == 0 {
		alerts = []models.DbAlert{}
	} 

    c.JSON(http.StatusOK, gin.H{
        "data":     alerts ,
        "totalRowCount":  count,
    })
}

func AddComment(c *gin.Context) {

    id := c.Param("id")
    // Convert string ID to BSON ObjectID
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }
    collection := db.GetCollection("alerts")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cur, err := collection.Find(ctx, bson.M{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer cur.Close(ctx)

    var newComment models.WorkLog
    if err := c.ShouldBindJSON(&newComment); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

    newComment.ID = primitive.NewObjectID()
    newComment.CreatedAt = time.Now()
    newComment.Author = username.(string)

    filter := bson.M{"_id": objectID}
    update := bson.M{"$push": bson.M{"worklogs": newComment}}

    _, err = collection.UpdateOne(context.TODO(), filter, update)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, newComment)
}

func Clear(c *gin.Context) {

    id := c.Param("id")
    // Convert string ID to BSON ObjectID
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }
    collection := db.GetCollection("alerts")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cur, err := collection.Find(ctx, bson.M{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer cur.Close(ctx)

    var newComment models.WorkLog
    if err := c.ShouldBindJSON(&newComment); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

    newComment.ID = primitive.NewObjectID()
    newComment.CreatedAt = time.Now()
    newComment.Author = username.(string)

    filter := bson.M{"_id": objectID}
    update := bson.M{
        "$push": bson.M{
            "worklogs": newComment,
            },
        "$set": bson.M{
            "alertstatus": "CLOSED",
        }}
    _, err = collection.UpdateOne(context.TODO(), filter, update)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, newComment)
}

func Notify(c *gin.Context) {
const NoderedEndpoint = "http://192.168.1.201:1880/notifications"
    id := c.Param("id")
    notificationid := c.Param("notificationid")
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }
    // Convert string ID to BSON ObjectID
    notificationobjectID, err := primitive.ObjectIDFromHex(notificationid)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid Notification ID format"})
        return
    }
    collection := db.GetCollection("alerts")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    var record models.DbAlert
    err1 := collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&record)
    if err1 != nil {
        fmt.Println("Error is ", err1)
        c.JSON(http.StatusNotFound, gin.H{"message": "Item not found"})
        return
    }
    collection = db.GetCollection("notifyrules")
    var notifyrecord models.DbNotifyRule
    err2 := collection.FindOne(context.Background(), bson.M{"_id": notificationobjectID}).Decode(&notifyrecord)
    if err2 != nil {
        fmt.Println("Error is ", err2)
        c.JSON(http.StatusNotFound, gin.H{"message": "Item not found"})
        return
    }
    record.AlertDestination = notifyrecord.RuleName
    // Convert DbAlert to byte
    byteSlice, err := json.Marshal(record)
    if err != nil {
        fmt.Println("Error:", err)
    }
    fmt.Println(byteSlice)
    response, err := http.Post( NoderedEndpoint , "application/json", bytes.NewBuffer(byteSlice))
    if err != nil {
        log.Fatalf("Error making POST request: %v", err)
    }
    defer response.Body.Close()
    
    body, err := io.ReadAll(response.Body)
    if err != nil {
        log.Fatalf("Error reading response body: %v", err)
    }
    fmt.Println(string(body))

    cur, err := collection.Find(ctx, bson.M{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer cur.Close(ctx)

    var newComment models.WorkLog
    if err := c.ShouldBindJSON(&newComment); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

    newComment.ID = primitive.NewObjectID()
    newComment.CreatedAt = time.Now()
    newComment.Author = username.(string)
    collection = db.GetCollection("alerts")
    filter := bson.M{"_id": objectID}
    update := bson.M{
        "$push": bson.M{
            "worklogs": newComment,
            },
        }
    _, err = collection.UpdateOne(context.TODO(), filter, update)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, newComment)
}

func Acknowledge(c *gin.Context) {

    id := c.Param("id")
    // Convert string ID to BSON ObjectID
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }
    collection := db.GetCollection("alerts")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cur, err := collection.Find(ctx, bson.M{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer cur.Close(ctx)

    var newComment models.WorkLog
    if err := c.ShouldBindJSON(&newComment); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

    newComment.ID = primitive.NewObjectID()
    newComment.CreatedAt = time.Now()
    newComment.Author = username.(string)

    filter := bson.M{"_id": objectID}
    update := bson.M{
        "$push": bson.M{
            "worklogs": newComment,
            },
        "$set": bson.M{
            "alertacked": "YES",
        }}

    _, err = collection.UpdateOne(context.TODO(), filter, update)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, newComment)
}

func Unacknowledge(c *gin.Context) {

    id := c.Param("id")
    // Convert string ID to BSON ObjectID
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }
    collection := db.GetCollection("alerts")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cur, err := collection.Find(ctx, bson.M{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer cur.Close(ctx)



    var newComment models.WorkLog
    if err := c.ShouldBindJSON(&newComment); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

    newComment.ID = primitive.NewObjectID()
    newComment.CreatedAt = time.Now()
    newComment.Author = username.(string)

    filter := bson.M{"_id": objectID}
    update := bson.M{
        "$push": bson.M{
            "worklogs": newComment,
            },
        "$set": bson.M{
            "alertacked": "NO",
        }}

    _, err = collection.UpdateOne(context.TODO(), filter, update)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, newComment)
}


// Handler function to get a record to edit.
func View(c *gin.Context) {

    id := c.Param("id")
    // Convert string ID to BSON ObjectID
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }
    collection := db.GetCollection("alerts")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cur, err := collection.Find(ctx, bson.M{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer cur.Close(ctx)

    var record models.DbAlert
    err1 := collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&record)
    if err1 != nil {
        fmt.Println("Error is ", err1)
        c.JSON(http.StatusNotFound, gin.H{"message": "Item not found"})
        return
    }

    if record.Parent {
        
        filter := bson.M{"_id": bson.M{"$in": record.GroupAlerts}}
        // Find documents matching the filter
        cursor, err := collection.Find(context.Background(), filter)
        if err != nil {
            fmt.Println("error1")
            return 
        }
    
        defer cursor.Close(context.Background())

        // Define a slice to store the results
        var alerts []models.DbAlert
    
        // Iterate over the cursor and decode each document into the User struct
        for cursor.Next(context.Background()) {
            var alert models.DbAlert
            err := cursor.Decode(&alert)
            if err != nil {
                fmt.Println("Error decoding document:", err)
                continue
            }
            alerts = append(alerts, alert)
        }
    
        // Check for any errors during cursor iteration
        if err := cursor.Err(); err != nil {
            fmt.Println("Error iterating cursor:", err)
            return
        }
    
        // Print the retrieved users
        fmt.Println("Retrieved Alerts:", alerts)
       record.ChildAlerts = alerts

    }

    c.JSON(http.StatusOK, record)

}

// Handler function to update a record from callback.
func AlertCallback(c *gin.Context) {
    var callbackData map[string]interface{}
    
    if err := c.ShouldBindJSON(&callbackData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
    fmt.Println("The callback is ", callbackData["mongoID"])

    id := callbackData["mongoID"].(string)
    // Convert string ID to BSON ObjectID
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid ID format"})
        return
    }

    collection := db.GetCollection("alerts")
    fmt.Println("callbackData[mongoID]", callbackData["mongoID"])
	updatefilter := bson.M{"_id": objectID }
    // Prepare the update document using the $set operator
    var newComment models.WorkLog
    newComment.ID = primitive.NewObjectID()
    newComment.CreatedAt = time.Now()
    newComment.Author = "System"
    newComment.Comment = fmt.Sprintf("Notification created in %v", callbackData["itsmtool"])
    update := bson.M{
        "$push": bson.M{
            "worklogs": newComment,
            },
        "$set": bson.M{
            "additionalDetails.ticket": callbackData["ticketNumber"].(string),
        }}
    updateResult , updateerr := collection.UpdateOne(context.TODO(), updatefilter, update)
    if updateerr != nil {
        panic(updateerr)
    }
    fmt.Println("updateResult.ModifiedCount" , updateResult.ModifiedCount)
    if updateResult.ModifiedCount > 0 {
        fmt.Printf("Matched %v documents and updated %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
    }
	c.JSON(http.StatusOK, gin.H{"modified": updateResult.ModifiedCount})
}

