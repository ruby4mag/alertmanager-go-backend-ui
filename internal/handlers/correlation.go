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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
)

// ---------------------------------------------------------------------------
// DATA STRUCTURES
// ---------------------------------------------------------------------------

type AlertDetail struct {
	Summary   string `json:"summary,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Status    string `json:"status,omitempty"`
	Severity  string `json:"severity,omitempty"`
	Priority  string `json:"priority,omitempty"`
	AlertID   string `json:"alert_id,omitempty"`
	FirstSeen string `json:"first_seen,omitempty"`
	LastSeen  string `json:"last_seen,omitempty"`
}

type Node struct {
	Name     string        `json:"name"`
	HasAlert bool          `json:"has_alert"`
	Severity string        `json:"severity,omitempty"` // Highest severity
	Alerts   []AlertDetail `json:"alerts,omitempty"`   // Multiple alerts on the same entity
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

// ---------------------------------------------------------------------------
// NEO4J DRIVER INIT
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// HTTP HANDLER
// ---------------------------------------------------------------------------

func HandleEntityGraph(c *gin.Context) {
	root := c.Param("name")

	graph, err := BuildEntityGraph(root)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, graph)
}

// ---------------------------------------------------------------------------
// HELPER — Severity ranking
// ---------------------------------------------------------------------------

func pickHighestSeverity(a, b string) string {
	order := map[string]int{
		"INFO":     1,
		"WARN":     2,
		"ERROR":    3,
		"CRITICAL": 4,
	}

	if order[b] > order[a] {
		return b
	}
	return a
}

// ---------------------------------------------------------------------------
// HELPER — Fetch ALL alerts for a node from MongoDB
// ---------------------------------------------------------------------------

func fetchNodeAlerts(node string) ([]AlertDetail, string) {

	col := db.GetCollection("alerts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"$or": []bson.M{
			{"entity": node},
			{"host": node},
		},
	}

	cursor, err := col.Find(ctx, filter)
	if err != nil {
		return nil, ""
	}
	defer cursor.Close(ctx)

	alerts := []AlertDetail{}
	highestSeverity := ""

	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		getString := func(key string) string {
			if v, ok := doc[key].(string); ok {
				return v
			}
			return ""
		}

		parseTime := func(field string) string {
			m1, ok := doc[field].(bson.M)
			if !ok {
				return ""
			}
			m2, ok := m1["time"].(bson.M)
			if !ok {
				return ""
			}
			if dt, ok := m2["$date"].(primitive.DateTime); ok {
				return dt.Time().Format(time.RFC3339)
			}
			return ""
		}

		alert := AlertDetail{
			Summary:   getString("alertsummary"),
			Notes:     getString("alertnotes"),
			Status:    getString("alertstatus"),
			Severity:  getString("severity"),
			Priority:  getString("alertpriority"),
			AlertID:   getString("alertid"),
			FirstSeen: parseTime("alertfirsttime"),
			LastSeen:  parseTime("alertlasttime"),
		}

		alerts = append(alerts, alert)
		highestSeverity = pickHighestSeverity(highestSeverity, alert.Severity)
	}

	return alerts, highestSeverity
}

// ---------------------------------------------------------------------------
// HELPER — Unique edge list
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// HELPER — Unique nodes
// ---------------------------------------------------------------------------

func uniqueNodes(include map[string]struct{}, alertMap map[string][]AlertDetail, severityMap map[string]string) []Node {
	out := []Node{}

	for name := range include {
		alerts := alertMap[name]
		sev := severityMap[name]

		out = append(out, Node{
			Name:     name,
			HasAlert: len(alerts) > 0,
			Severity: sev,
			Alerts:   alerts,
		})
	}

	return out
}

// ---------------------------------------------------------------------------
// MAIN FUNCTION — Build alert subgraph starting from root node
// ---------------------------------------------------------------------------

func BuildEntityGraph(root string) (*GraphResponse, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session := neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	// -----------------------------------------------------------------------
	// 1. Fetch root + 6-hop paths
	// -----------------------------------------------------------------------
	cypher := `
	MATCH (root {name:$root})
	OPTIONAL MATCH p = (root)-[*..6]-(n)
	RETURN 
		COLLECT(DISTINCT root.name),
		COLLECT(DISTINCT n.name),
		COLLECT(p)
	`

	result, err := session.Run(ctx, cypher, map[string]interface{}{"root": root})
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

	allNodes := map[string]struct{}{}

	for _, n := range rawRootNodes {
		allNodes[n.(string)] = struct{}{}
	}
	for _, n := range rawOtherNodes {
		allNodes[n.(string)] = struct{}{}
	}

	// -----------------------------------------------------------------------
	// 2. Fetch alerts for all nodes
	// -----------------------------------------------------------------------
	alertSet := map[string]struct{}{}
	alertMap := map[string][]AlertDetail{}
	severityMap := map[string]string{}

	for name := range allNodes {
		alerts, severity := fetchNodeAlerts(name)
		alertMap[name] = alerts
		severityMap[name] = severity

		if len(alerts) > 0 {
			alertSet[name] = struct{}{}
		}
	}

	// -----------------------------------------------------------------------
	// 3. Parse Neo4j paths into edges
	// -----------------------------------------------------------------------
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

		for _, n := range path.Nodes {
			idToName[n.ElementId] = n.Props["name"].(string)
		}

		for _, r := range path.Relationships {
			rawEdges = append(rawEdges, edgeTuple{
				srcID: r.StartElementId,
				tgtID: r.EndElementId,
				typ:   r.Type,
			})
		}
	}

	edges := []Edge{}
	for _, e := range rawEdges {
		src := idToName[e.srcID]
		tgt := idToName[e.tgtID]
		if src != "" && tgt != "" {
			edges = append(edges, Edge{Source: src, Target: tgt, Type: e.typ})
		}
	}

	// -----------------------------------------------------------------------
	// 4. Determine which nodes to include:
	//    - alert nodes
	//    - nodes on path root → alert nodes
	//    - nodes on path alert ↔ alert
	// -----------------------------------------------------------------------
	include := map[string]struct{}{}

	for a := range alertSet {
		include[a] = struct{}{}
	}

	includePath := func(a, b string) {
		q := `
		MATCH p = shortestPath((x {name:$a})-[*..6]-(y {name:$b}))
		RETURN [n IN nodes(p) | n.name]
		`
		run, err := session.Run(ctx, q, map[string]interface{}{"a": a, "b": b})
		if err != nil {
			return
		}
		if run.Next(ctx) {
			arr, ok := run.Record().Values[0].([]interface{})
			if ok {
				for _, x := range arr {
					include[x.(string)] = struct{}{}
				}
			}
		}
	}

	// root → each alert node
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

	// -----------------------------------------------------------------------
	// 5. Filter edges and nodes
	// -----------------------------------------------------------------------
	filteredEdges := []Edge{}
	for _, e := range edges {
		if _, ok := include[e.Source]; ok {
			if _, ok2 := include[e.Target]; ok2 {
				filteredEdges = append(filteredEdges, e)
			}
		}
	}

	finalEdges := uniqueEdges(filteredEdges)
	finalNodes := uniqueNodes(include, alertMap, severityMap)

	// -----------------------------------------------------------------------
	// RETURN RESULT
	// -----------------------------------------------------------------------

	return &GraphResponse{
		Root:  root,
		Nodes: finalNodes,
		Edges: finalEdges,
	}, nil
}

// ---------------------------------------------------------------------------
// small helper
// ---------------------------------------------------------------------------

func keys(m map[string]struct{}) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}
