package db

import (
	"log"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

var Neo4jDriver neo4j.DriverWithContext

func InitNeo4j(uri, username, password string) {
	var err error
	Neo4jDriver, err = neo4j.NewDriverWithContext(
		uri,
		neo4j.BasicAuth(username, password, ""),
		func(c *neo4j.Config) {
			c.MaxConnectionLifetime = 5 * time.Minute
			c.MaxConnectionPoolSize = 50
			c.ConnectionAcquisitionTimeout = 10 * time.Second
		},
	)
	if err != nil {
		log.Fatalf("Neo4j connection failed: %v", err)
	}
	log.Println("Neo4j driver initialized successfully")
}

func GetNeo4jDriver() neo4j.DriverWithContext {
	return Neo4jDriver
}
