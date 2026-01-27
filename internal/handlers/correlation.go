package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
    "github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
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

// Update Node struct definition
type Node struct {
	Name         string           `json:"name"`
	HasAlert     bool             `json:"has_alert"`
	Severity     string           `json:"severity,omitempty"`
	SupportOwner string           `json:"support_owner,omitempty"`
	Alerts       []AlertDetail    `json:"alerts,omitempty"`
	Changes      []models.RelatedChange `json:"changes"` // Change info
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

// ---------------------------------------------------------------------------
// HELPER — Fetch ALL changes for a node from MongoDB
// ---------------------------------------------------------------------------

func fetchNodeChanges(node string) []models.RelatedChange {
    col := db.GetCollection("changes")
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Find changes affecting this entity within recent window (e.g. last 24h or fixed window)
    // For now, let's just fetch recent changes (last 7 days?) to be safe, or just all for demo context
    // Ideally we should use the time window from the context if available, but for Entity Graph it's generic discovery.
    // Let's get last 24h changes.
    // For demo purposes/testing with mismatched sample dates, we are removing the time filter.
    // In production, this should likely be -7 days or similar.
    filter := bson.M{
        "affected_entities": node,
    }

    // Check filter correctness
    // log.Printf("DEBUG: Fetching changes for node: %s since %v", node, startTime)

    cursor, err := col.Find(ctx, filter)
    if err != nil {
        log.Printf("DEBUG: Find error for node %s: %v", node, err)
        return nil
    }
    defer cursor.Close(ctx)

    var dbChanges []models.Change
    if err := cursor.All(ctx, &dbChanges); err != nil {
        log.Printf("DEBUG: Decode error for node %s: %v", node, err)
        return nil
    }
    
    // if len(dbChanges) > 0 {
    //    log.Printf("DEBUG: Found %d changes for node %s", len(dbChanges), node)
    // }

    changes := []models.RelatedChange{}
    for _, ch := range dbChanges {
        changes = append(changes, models.RelatedChange{
            ChangeID:      ch.ChangeID,
            Name:          ch.Name,
            ChangeType:    ch.ChangeType,
            Status:        ch.Status,
            ImplementedBy: ch.ImplementedBy,
            StartTime:     ch.StartTime,
            EndTime:       ch.EndTime,
            // OverlapType is relative to an alert, so it might be null here or generic
            ChangeScope:   "direct", // On this specific node, it is direct
        })
    }
    return changes
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

func BuildEntityGraph(root string) (*GraphResponse, error) { 
	log.Printf("DEBUG: Searching for root node: '%s' (len: %d)", root, len(root))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session := db.GetNeo4jDriver().NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	_, err := session.Run(ctx, "RETURN 1", nil)
	if err != nil {
		log.Printf("DEBUG: Connectivity Check Failed! Could not run simple query: %v", err)
		return nil, err
	}
	log.Println("DEBUG: Neo4j Connection Verified. Proceeding with Graph Query...")

	cypher := `
	MATCH (root) 
	WHERE root.name = $root OR root.id = $root
	
	CALL {
		WITH root
		MATCH (root)-[*0..10]-(n)
		RETURN collect(DISTINCT n) as nodes
	}

	WITH nodes
	UNWIND nodes as n
	MATCH (n)-[r]-(m)
	WHERE m IN nodes
	RETURN nodes as unique_nodes, collect(DISTINCT r) as unique_edges
	`

	result, err := session.Run(ctx, cypher, map[string]interface{}{"root": root})
	if err != nil {
		log.Printf("DEBUG: Neo4j Session Run failed: %v", err)
		return nil, err
	}
	if !result.Next(ctx) {
		log.Printf("DEBUG: Query result empty. Root node '%s' not found.", root)
		return nil, fmt.Errorf("root node not found: %s", root)
	}

	rec := result.Record()

	rawNodes := rec.Values[0].([]interface{})
	rawEdges := rec.Values[1].([]interface{})

	allNodes := map[string]struct{}{}
	idToName := map[string]string{}
	nodeProps := map[string]string{}

	for _, n := range rawNodes {
		node, ok := n.(dbtype.Node)
		if ok {
			if nameVal, exists := node.Props["name"]; exists {
				nameStr := nameVal.(string)
				allNodes[nameStr] = struct{}{}
				idToName[node.ElementId] = nameStr

				if owner, ok := node.Props["support_owner"].(string); ok {
					nodeProps[nameStr] = owner
				}
			}
		}
	}

	edges := []Edge{}
	for _, r := range rawEdges {
		rel, ok := r.(dbtype.Relationship)
		if ok {
			src := idToName[rel.StartElementId]
			tgt := idToName[rel.EndElementId]
			if src != "" && tgt != "" {
				edges = append(edges, Edge{Source: src, Target: tgt, Type: rel.Type})
			}
		}
	}

	alertSet := map[string]struct{}{}
	alertMap := map[string][]AlertDetail{}
	changeMap := map[string][]models.RelatedChange{} // Store changes per node
	severityMap := map[string]string{}

	for name := range allNodes {
		alerts, severity := fetchNodeAlerts(name)
		changes := fetchNodeChanges(name) // Fetch Changes
		
		alertMap[name] = alerts
		changeMap[name] = changes
		severityMap[name] = severity

		if len(alerts) > 0 {
			alertSet[name] = struct{}{}
		}
        // Also include nodes if they have recent changes? 
        // Requirement says "add the nodes with direct changed ... so i can show visually".
        // It implies if a node has changes, it might be interesting even if no alert?
        // But the previous logic was aggressively pruning nodes without alerts.
        // Let's keep the existing logic: include path to alerts.
        // If a node on the path has changes, we show them.
        // If the user wants to see *all* nodes with changes, we should add them to 'include' set.
        if len(changes) > 0 {
             alertSet[name] = struct{}{} // Treat "Has Change" similar to "Has Alert" for graph inclusion
        }
	}
	log.Printf("DEBUG: Alert/Change Lookup -> Checked %d nodes. Interesting Nodes: %d", len(allNodes), len(alertSet))

	include := map[string]struct{}{}

	for a := range alertSet {
		include[a] = struct{}{}
	}

	includePath := func(a, b string) {
		if a == b {
			return
		}
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

	for a := range alertSet {
		includePath(root, a)
	}

	alertList := keys(alertSet)
	for i := 0; i < len(alertList); i++ {
		for j := i + 1; j < len(alertList); j++ {
			includePath(alertList[i], alertList[j])
		}
	}

	filteredEdges := []Edge{}
	for _, e := range edges {
		if _, ok := include[e.Source]; ok {
			if _, ok2 := include[e.Target]; ok2 {
				filteredEdges = append(filteredEdges, e)
			}
		}
	}

	finalEdges := uniqueEdges(filteredEdges)
	finalNodes := uniqueNodes(include, alertMap, changeMap, severityMap, nodeProps) // Passed changeMap

	return &GraphResponse{
		Root:  root,
		Nodes: finalNodes,
		Edges: finalEdges,
	}, nil
}

func uniqueNodes(include map[string]struct{}, alertMap map[string][]AlertDetail, changeMap map[string][]models.RelatedChange, severityMap map[string]string, nodeProps map[string]string) []Node {
	out := []Node{}

	for name := range include {
		alerts := alertMap[name]
		changes := changeMap[name]
		sev := severityMap[name]
		owner := nodeProps[name]

		out = append(out, Node{
			Name:         name,
			HasAlert:     len(alerts) > 0,
			Severity:     sev,
			Alerts:       alerts,
			Changes:      changes, // Populate Changes
			SupportOwner: owner,
		})
	}

	return out
}

func keys(m map[string]struct{}) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}
