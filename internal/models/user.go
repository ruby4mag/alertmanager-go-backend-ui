package models

import (
	"context"
	"time"

	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       	string `bson:"_id,omitempty" json:"id,omitempty"`
	Username 	string `bson:"username" json:"username"`
	Email 		string `bson:"email" json:"email"`
	Role 		string `bson:"role" json:"role"`
	Password 	string `bson:"password,omitempty" json:"password"`
}

func (u *User) HashPassword(password string) error {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		return err
	}
	u.Password = string(bytes)
	return nil
}

func (u *User) CheckPassword(providedPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(providedPassword))
}

func FindUserByUsername(username string) (*User, error) {
	var user User
	collection := db.GetCollection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func CreateUser(user *User) error {
	collection := db.GetCollection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, user)
	return err
}
