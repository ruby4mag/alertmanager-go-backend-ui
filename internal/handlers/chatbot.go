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
)

// ChatbotProxy handles chatbot requests and injects sessionId before forwarding to n8n
func ChatbotProxy(c *gin.Context) {
	// Get n8n webhook URL from environment or use default
	n8nWebhookURL := os.Getenv("N8N_CHAT_WEBHOOK_URL")
	if n8nWebhookURL == "" {
		n8nWebhookURL = "http://192.168.1.201:5678/webhook/alert-chat"
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

	// Read the response from n8n
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading n8n response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read n8n response"})
		return
	}

	// Forward the response back to the client
	// Check if the response is JSON
	var jsonResponse interface{}
	if err := json.Unmarshal(responseBody, &jsonResponse); err == nil {
		// It's valid JSON, send it as JSON
		c.JSON(resp.StatusCode, jsonResponse)
	} else {
		// Not JSON, send as plain text
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), responseBody)
	}

	log.Printf("Chatbot request forwarded to n8n with sessionId: %s, status: %d", sessionID, resp.StatusCode)
}

// ChatbotStream handles streaming chatbot responses (for SSE/streaming)
func ChatbotStream(c *gin.Context) {
	// Get n8n webhook URL from environment or use default
	n8nWebhookURL := os.Getenv("N8N_CHAT_WEBHOOK_URL")
	if n8nWebhookURL == "" {
		n8nWebhookURL = "http://192.168.1.201:5678/webhook/alert-chat"
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
