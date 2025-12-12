package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
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

var neo4jDriver neo4j.DriverWithContext

func init() {
	var err error
	neo4jDriver, err = neo4j.NewDriverWithContext(
		"bolt://192.168.1.201:7687",
		neo4j.BasicAuth("neo4j", "kl8j2300", ""),
	)
	if err != nil {
		log.Fatalf("Neo4j connection failed: %v", err)
	}
}

func HandleEntityGraph(c *gin.Context) {
	root := c.Param("name")

	graph, err := BuildEntityGraph(root)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, graph)
}

// -----------------------------------------------------------------------------
// MAIN FUNCTION: Build minimal root-scoped alert subgraph
// -----------------------------------------------------------------------------

func BuildEntityGraph(root string) (*GraphResponse, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session := neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	// -------------------------------------------------------------------------
	// 1. GET ROOT + 6-HOP NEIGHBORHOOD
	// -------------------------------------------------------------------------
	query := `
	MATCH (root {name:$root})
	OPTIONAL MATCH p = (root)-[*..6]-(n)
	RETURN 
		COLLECT(DISTINCT root.name) AS rootNodes,
		COLLECT(DISTINCT n.name) AS otherNodes,
		COLLECT(p) AS paths
	`

	result, err := session.Run(ctx, query, map[string]interface{}{"root": root})
	if err != nil {
		return nil, err
	}

	if !result.Next(ctx) {
		return nil, fmt.Errorf("root node not found: %s", root)
	}

	rec := result.Record()

	rawRootNodes := rec.Values[0].([]interface{})
	rawOtherNodes := rec.Values[1].([]interface{})
	rawPaths := rec.Values[2].([]interface{})

	// Build node name set
	allNodes := map[string]struct{}{}
	for _, rn := range rawRootNodes {
		allNodes[rn.(string)] = struct{}{}
	}
	for _, rn := range rawOtherNodes {
		allNodes[rn.(string)] = struct{}{}
	}

	// -------------------------------------------------------------------------
	// 2. DETERMINE NODES WITH ALERTS
	// -------------------------------------------------------------------------
	alertSet := map[string]struct{}{}
	alertInfo := map[string]AlertInfo{}

	for name := range allNodes {
		ai := fetchAlertInfo(name)
		alertInfo[name] = ai
		if ai.HasAlert {
			alertSet[name] = struct{}{}
		}
	}

	// -------------------------------------------------------------------------
	// 3. PARSE RAW PATHS → COLLECT EDGES + ID→NAME MAP
	// -------------------------------------------------------------------------
	type edgeTuple struct {
		srcID string
		tgtID string
		typ   string
	}

	rawEdges := []edgeTuple{}
	idToName := map[string]string{}

	for _, p := range rawPaths {
		path, ok := p.(dbtype.Path)
		if !ok {
			continue
		}

		// Map node IDs
		for _, n := range path.Nodes {
			idToName[n.ElementId] = n.Props["name"].(string)
		}

		// Extract edges
		for _, r := range path.Relationships {
			rawEdges = append(rawEdges, edgeTuple{
				srcID: r.StartElementId,
				tgtID: r.EndElementId,
				typ:   r.Type,
			})
		}
	}

	// Convert ID→Name edges
	edges := []Edge{}
	for _, e := range rawEdges {
		src := idToName[e.srcID]
		tgt := idToName[e.tgtID]
		if src != "" && tgt != "" {
			edges = append(edges, Edge{Source: src, Target: tgt, Type: e.typ})
		}
	}

	// -------------------------------------------------------------------------
	// 4. DETERMINE WHICH NODES SHOULD BE INCLUDED
	// -------------------------------------------------------------------------
	include := map[string]struct{}{}

	// always include alert nodes
	for a := range alertSet {
		include[a] = struct{}{}
	}

	// shortest-path resolver
	includePath := func(a, b string) {
		q := `
		MATCH p = shortestPath((x {name:$a})-[*..6]-(y {name:$b}))
		RETURN [n IN nodes(p) | n.name] AS ns
		`
		res, err := session.Run(ctx, q, map[string]interface{}{"a": a, "b": b})
		if err != nil {
			return
		}
		if res.Next(ctx) {
			arr, ok := res.Record().Values[0].([]interface{})
			if ok {
				for _, v := range arr {
					include[v.(string)] = struct{}{}
				}
			}
		}
	}

	// root → alert
	for a := range alertSet {
		includePath(root, a)
	}

	// alert ↔ alert
	alertList := keys(alertSet)
	for i := 0; i < len(alertList); i++ {
		for j := i + 1; j < len(alertList); j++ {
			includePath(alertList[i], alertList[j])
		}
	}

	// -------------------------------------------------------------------------
	// 5. FILTER EDGES BY INCLUDED NODES
	// -------------------------------------------------------------------------
	filteredEdges := []Edge{}
	for _, e := range edges {
		if _, ok := include[e.Source]; ok {
			if _, ok := include[e.Target]; ok {
				filteredEdges = append(filteredEdges, e)
			}
		}
	}

	// -------------------------------------------------------------------------
	// 6. REMOVE DUPLICATE EDGES
	// -------------------------------------------------------------------------
	finalEdges := uniqueEdges(filteredEdges)

	// -------------------------------------------------------------------------
	// 7. REMOVE DUPLICATE NODES
	// -------------------------------------------------------------------------
	finalNodes := uniqueNodes(include, alertInfo)

	// -------------------------------------------------------------------------
	// RETURN FINAL RESULT
	// -------------------------------------------------------------------------
	return &GraphResponse{
		Root:  root,
		Nodes: finalNodes,
		Edges: finalEdges,
	}, nil
}

// -----------------------------------------------------------------------------
// HELPERS
// -----------------------------------------------------------------------------

// Remove duplicate edges
func uniqueEdges(edges []Edge) []Edge {
	seen := map[string]struct{}{}
	out := []Edge{}

	for _, e := range edges {
		key := e.Source + "|" + e.Target + "|" + e.Type
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e)
	}
	return out
}

// Remove duplicate nodes (only one per name)
func uniqueNodes(include map[string]struct{}, alertInfo map[string]AlertInfo) []Node {
	out := []Node{}
	for name := range include {
		ai := alertInfo[name]
		out = append(out, Node{
			Name:     name,
			HasAlert: ai.HasAlert,
			Severity: ai.Severity,
		})
	}
	return out
}

// MongoDB alert lookup
func fetchAlertInfo(node string) AlertInfo {
	col := db.GetCollection("alerts")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"$or": []bson.M{
			{"host": node},
			{"entity": node},
		},
	}

	var doc bson.M
	err := col.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		return AlertInfo{HasAlert: false}
	}

	sev := ""
	if s, ok := doc["severity"].(string); ok {
		sev = s
	}

	return AlertInfo{
		HasAlert: true,
		Severity: sev,
	}
}

func keys(m map[string]struct{}) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}
