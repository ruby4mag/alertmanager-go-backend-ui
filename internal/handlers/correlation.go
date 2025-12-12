package handlers

import (
    "context"
    "fmt"
    "net/http"
    "time"
	"log"

    "github.com/gin-gonic/gin"
    "github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
    "go.mongodb.org/mongo-driver/bson"
    "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type AlertInfo struct {
    HasAlert bool   `json:"has_alert"`
    Severity string `json:"severity,omitempty"`
}

type Node struct {
    Name     string `json:"name"`
    HasAlert bool   `json:"has_alert"`
    Severity string `json:"severity,omitempty"`
}

type Edge struct {
    Source string `json:"source"`
    Target string `json:"target"`
    Type   string `json:"type"`
}

type GraphResponse struct {
    Root  string `json:"root"`
    Nodes []Node `json:"nodes"`
    Edges []Edge `json:"edges"`
}

// ----------------------------
// Connect to Neo4j
// ----------------------------
func init() {
    var err error
    neo4jDriver, err = neo4j.NewDriverWithContext(
        "bolt://192.168.1.201:7687",
        neo4j.BasicAuth("neo4j", "kl8j2300", ""),
    )
    if err != nil {
        log.Fatal("Neo4j Connection Error:", err)
    }
}

var neo4jDriver neo4j.DriverWithContext

// Handler function to fetch entity graph
func HandleEntityGraph(c *gin.Context) {
    entity := c.Param("name")

    graph, err := BuildEntityGraph(entity)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, graph)
}

func BuildEntityGraph(entity string) (*GraphResponse, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    session := neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
    defer session.Close(ctx)

    // ----------------------------
    // Neo4j 2-hop Query
    // ----------------------------
    cypher := `
    MATCH (root {name: $name})
    OPTIONAL MATCH p = (root)-[*..6]-(n)
    WITH COLLECT(DISTINCT root) + COLLECT(DISTINCT n) AS allNodes, COLLECT(DISTINCT p) AS allPaths
    UNWIND allNodes AS nd
    RETURN 
       COLLECT(DISTINCT {name: nd.name}) AS nodes,
       [p IN allPaths |
          [r IN relationships(p) | {source: startNode(r).name, target: endNode(r).name, type: type(r)}]
       ] AS edges
    `

result, err := session.Run(ctx, cypher, map[string]interface{}{"name": entity})
if err != nil {
    return nil, err
}

var nodesRaw []map[string]interface{}
var edgesRaw [][]map[string]interface{}

if result.Next(ctx) {
    rec := result.Record()

    // -------------------------
    // NODE UNMARSHALLING
    // -------------------------
    rawNodes, ok := rec.Values[0].([]interface{})
    if !ok {
        return nil, fmt.Errorf("unexpected type for nodes: %T", rec.Values[0])
    }

    nodesRaw = make([]map[string]interface{}, 0)
    for _, item := range rawNodes {
        if nodeMap, ok := item.(map[string]interface{}); ok {
            nodesRaw = append(nodesRaw, nodeMap)
        }
    }

    // -------------------------
    // EDGE UNMARSHALLING
    // -------------------------
    rawEdges, ok := rec.Values[1].([]interface{})
    if !ok {
        return nil, fmt.Errorf("unexpected type for edges: %T", rec.Values[1])
    }

    edgesRaw = make([][]map[string]interface{}, 0)

    for _, pathItem := range rawEdges {
        pathList, ok := pathItem.([]interface{})
        if !ok {
            continue
        }

        edgeList := make([]map[string]interface{}, 0)
        for _, e := range pathList {
            if eMap, ok := e.(map[string]interface{}); ok {
                edgeList = append(edgeList, eMap)
            }
        }

        edgesRaw = append(edgesRaw, edgeList)
    }
}

// -------------------------
// FLATTEN EDGES (GraphFormat)
// -------------------------
edges := []Edge{}
for _, pathEdges := range edgesRaw {
    for _, e := range pathEdges {
        src, _ := e["source"].(string)
        tgt, _ := e["target"].(string)
        typ, _ := e["type"].(string)

        edges = append(edges, Edge{
            Source: src,
            Target: tgt,
            Type:   typ,
        })
    }
}


    // Convert nodes + check alerts
    nodes := []Node{}
    for _, n := range nodesRaw {
        name := n["name"].(string)
        alertInfo := fetchAlertInfo(name)

        nodes = append(nodes, Node{
            Name:     name,
            HasAlert: alertInfo.HasAlert,
            Severity: alertInfo.Severity,
        })
    }

    return &GraphResponse{
        Root:  entity,
        Nodes: nodes,
        Edges: edges,
    }, nil
}

func fetchAlertInfo(node string) AlertInfo {
    collection := db.GetCollection("alerts")
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    filter := bson.M{
        "$or": []bson.M{
            {"host": node},
            {"entity": node},
        },
    }

    var result bson.M
    err := collection.FindOne(ctx, filter).Decode(&result)

    if err != nil {
        return AlertInfo{HasAlert: false}
    }

    severity, ok := result["severity"].(string)
    if !ok {
        severity = ""
    }

    return AlertInfo{
        HasAlert: true,
        Severity: severity,
    }
}
