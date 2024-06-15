package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/gin-gonic/gin"
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

    filter := bson.M{}
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
	collection := db.GetCollection("users")

	cursor, err := collection.Find(ctx, filter, findOptions)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    var users []models.User
    if err := cursor.All(ctx, &users); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }



    count, err := collection.CountDocuments(ctx, filter)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

	if len(users) == 0 {
		users = []models.User{}
	} 

    c.JSON(http.StatusOK, gin.H{
        "data":     users ,
        "totalRowCount":  count,
    })









	/////////////////////////////////////////////

   
}

