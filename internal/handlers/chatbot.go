package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
	"github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ChatbotProxy handles chatbot requests and injects sessionId before forwarding to n8n
func ChatbotProxy(c *gin.Context) {
	// Get n8n webhook URL from environment or use default
	n8nWebhookURL := os.Getenv("N8N_CHAT_WEBHOOK_URL")
	if n8nWebhookURL == "" {
		n8nWebhookURL = "http://localhost:5678/webhook/alert-chat"
	}

	// Parse the incoming request body
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Extract alert data to get the incident ID
	var sessionID string
	if alert, ok := requestBody["alert"].(map[string]interface{}); ok {
		if alertID, ok := alert["alertid"].(string); ok {
			sessionID = alertID
		}
	}

	// If sessionId is not already present, inject it
	if sessionID != "" {
		requestBody["sessionId"] = sessionID
		log.Printf("Injected sessionId: %s into chatbot request", sessionID)

		// Check if action is "init"
		action, _ := requestBody["action"].(string)
		
		if action == "init" {
			if alertMap, ok := requestBody["alert"].(map[string]interface{}); ok {
				if _, hasGraphData := alertMap["graph_data"]; !hasGraphData {
					log.Printf("Graph data missing in alert for init action, generating for alert: %s", sessionID)
					
					// 1. Fetch Alert
					alertsCollection := db.GetCollection("alerts")
					var alert models.DbAlert
					
					// Try to find by string ID first
					objID, err := primitive.ObjectIDFromHex(sessionID)
					if err == nil {
						err = alertsCollection.FindOne(c.Request.Context(), bson.M{"_id": objID}).Decode(&alert)
					}
					
					if err != nil {
						err = alertsCollection.FindOne(c.Request.Context(), bson.M{"alertid": sessionID}).Decode(&alert)
					}

					if err == nil {
						// 2. Build Graph
						payload, err := BuildEntityGraph(alert.Entity)
						if err == nil {
							// 3. Inject into alert object
							alertMap["graph_data"] = payload
							// Re-assign to ensure map update is reflected (though map is ref type)
							requestBody["alert"] = alertMap
							
							log.Printf("Successfully injected graph data into alert object via BuildEntityGraph")
						} else {
							log.Printf("Error building Entity graph: %v", err)
						}
					} else {
						log.Printf("Error fetching alert for graph generation: %v", err)
					}
				}
			}
		}

	} else {
		log.Printf("Warning: Could not extract alert ID for sessionId")
	}

	// Marshal the modified request body
	modifiedBody, err := json.Marshal(requestBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process request"})
		return
	}

	// Forward the request to n8n
	req, err := http.NewRequest("POST", n8nWebhookURL, bytes.NewBuffer(modifiedBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request to n8n"})
		return
	}

	// Copy headers from original request
	req.Header.Set("Content-Type", "application/json")
	for key, values := range c.Request.Header {
		if key != "Content-Length" { // Skip Content-Length as it will be set automatically
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error forwarding request to n8n: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to reach n8n webhook"})
		return
	}
	defer resp.Body.Close()

	// Set response headers from n8n response
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	
	// Set status code
	c.Status(resp.StatusCode)

	// Stream the response directly without buffering
	// This enables real-time streaming to the frontend
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		log.Printf("Warning: ResponseWriter doesn't support flushing")
	}

	buffer := make([]byte, 4096) // 4KB buffer for efficient streaming
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			// Write chunk to response
			if _, writeErr := c.Writer.Write(buffer[:n]); writeErr != nil {
				log.Printf("Error writing chunk to client: %v", writeErr)
				return
			}
			
			// Flush immediately to send chunk to client
			if ok {
				flusher.Flush()
			}
		}
		
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading n8n response stream: %v", err)
			}
			break
		}
	}

	log.Printf("Chatbot request streamed to client with sessionId: %s, status: %d", sessionID, resp.StatusCode)
}

// ChatbotStream handles streaming chatbot responses (for SSE/streaming)
func ChatbotStream(c *gin.Context) {
	// Get n8n webhook URL from environment or use default
	n8nWebhookURL := os.Getenv("N8N_CHAT_WEBHOOK_URL")
	if n8nWebhookURL == "" {
		n8nWebhookURL = "http://localhost:5678/webhook/alert-chat"
	}

	// Parse the incoming request body
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Extract sessionId from request or alert data
	var sessionID string
	if sid, ok := requestBody["sessionId"].(string); ok && sid != "" {
		sessionID = sid
	} else if alert, ok := requestBody["alert"].(map[string]interface{}); ok {
		if alertID, ok := alert["alertid"].(string); ok {
			sessionID = alertID
			requestBody["sessionId"] = sessionID
		}
	}

	if sessionID == "" {
		log.Printf("Warning: No sessionId found in streaming request")
	} else {
		log.Printf("Processing streaming request with sessionId: %s", sessionID)


		// Check if action is "init"
		action, _ := requestBody["action"].(string)
		
		if action == "init" {
			if alertMap, ok := requestBody["alert"].(map[string]interface{}); ok {
				if _, hasGraphData := alertMap["graph_data"]; !hasGraphData {
					log.Printf("Graph data missing in alert for streaming init action, generating for alert: %s", sessionID)
					
					// 1. Fetch Alert
					alertsCollection := db.GetCollection("alerts")
					var alert models.DbAlert
					
					// Try to find by string ID first
					objID, err := primitive.ObjectIDFromHex(sessionID)
					if err == nil {
						err = alertsCollection.FindOne(c.Request.Context(), bson.M{"_id": objID}).Decode(&alert)
					}
					
					// If not found or error, try finding by alertid field
					if err != nil {
						err = alertsCollection.FindOne(c.Request.Context(), bson.M{"alertid": sessionID}).Decode(&alert)
					}

					if err == nil {
						// 2. Build Graph
						payload, err := BuildEntityGraph(alert.Entity)
						if err == nil {
							// 3. Inject into alert object
							alertMap["graph_data"] = payload
							// Re-assign to ensure map update is reflected
							requestBody["alert"] = alertMap
							
							log.Printf("Successfully injected graph data into alert object via BuildEntityGraph in streaming request")
						} else {
							log.Printf("Error building Entity graph: %v", err)
						}
					} else {
						log.Printf("Error fetching alert for graph generation: %v", err)
					}
				}
			}
		}
	}

	// Marshal the modified request body
	modifiedBody, err := json.Marshal(requestBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process request"})
		return
	}

	// Forward the request to n8n
	req, err := http.NewRequest("POST", n8nWebhookURL, bytes.NewBuffer(modifiedBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request to n8n"})
		return
	}

	// Set headers for streaming
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error forwarding streaming request to n8n: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to reach n8n webhook"})
		return
	}
	defer resp.Body.Close()

	// Set headers for SSE response
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Stream the response
	c.Stream(func(w io.Writer) bool {
		buffer := make([]byte, 1024)
		n, err := resp.Body.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading stream: %v", err)
			}
			return false
		}
		if n > 0 {
			fmt.Fprint(w, string(buffer[:n]))
		}
		return true
	})

	log.Printf("Streaming chatbot request completed for sessionId: %s", sessionID)
}

// ChatbotAction handles UI actions from the chatbot and forwards them to n8n
func ChatbotAction(c *gin.Context) {
	log.Printf("Received chatbot action request")
	n8nActionWebhookURL := os.Getenv("N8N_ACTION_WEBHOOK_URL")
	if n8nActionWebhookURL == "" {
		n8nActionWebhookURL = "http://localhost:5678/webhook/alert-action"
	}

	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Capture user who triggered the action
	username, _ := c.Get("username")
	payload["user"] = username

	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process request"})
		return
	}

	req, err := http.NewRequest("POST", n8nActionWebhookURL, bytes.NewBuffer(modifiedBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error forwarding action to n8n: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to reach n8n"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Post-processing: If creation was successful, update the alert in MongoDB
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var n8nResponse struct {
			PagerDutyResponse struct {
				Incident struct {
					IncidentNumber int    `json:"incident_number"`
					ID            string `json:"id"`
					HTMLURL       string `json:"html_url"`
					Status        string `json:"status"`
				} `json:"incident"`
			} `json:"pagerdutyResponse"`
		}

		if err := json.Unmarshal(body, &n8nResponse); err == nil {
			inc := n8nResponse.PagerDutyResponse.Incident
			if inc.ID != "" {
				// We have a successful PagerDuty incident creation
				alertIdStr, _ := payload["alertId"].(string)
				if alertIdStr != "" {
					log.Printf("Updating alert %s with Major Incident: %d (%s)", alertIdStr, inc.IncidentNumber, inc.Status)
					
					alertsCollection := db.GetCollection("alerts")
					update := bson.M{
						"$set": bson.M{
							"major_incident_number": inc.IncidentNumber,
							"major_incident_id":     inc.ID,
							"major_incident_url":    inc.HTMLURL,
							"major_incident_status": inc.Status,
						},
					}

					// Try updating by ObjectID or string alertid
					objID, err := primitive.ObjectIDFromHex(alertIdStr)
					if err == nil {
						alertsCollection.UpdateOne(c.Request.Context(), bson.M{"_id": objID}, update)
					} else {
						alertsCollection.UpdateOne(c.Request.Context(), bson.M{"alertid": alertIdStr}, update)
					}
				}
			}
		} else {
			log.Printf("Failed to unmarshal n8n response for post-processing: %v", err)
		}
	}

	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// Helper to convert alert struct to map
func alertMapFromStruct(alert models.DbAlert) map[string]interface{} {
	b, _ := json.Marshal(alert)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	return m
}
