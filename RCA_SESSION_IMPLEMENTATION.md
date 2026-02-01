# RCA SessionId Implementation

## Overview
This document describes the implementation of sessionId injection for n8n RCA workflow integration. The sessionId allows the n8n workflow to use Redis for maintaining conversation context specific to each incident.

## Changes Made

### 1. Updated RCA Graph Model (`internal/models/rca_graph.go`)
Added a `SessionID` field to the `RCAContext` struct:

```go
type RCAContext struct {
    AlertID      string    `json:"alert_id"`
    RootEntityID string    `json:"root_entity_id"`
    SessionID    string    `json:"sessionId"`    // Incident ID used for Redis session in n8n
    GeneratedAt  time.Time `json:"generated_at"`
}
```

**Key Points:**
- The field is named `sessionId` in JSON (camelCase) to match n8n's expected format
- This field will be used by the n8n workflow to set/retrieve context from Redis
- The sessionId is scoped per incident, allowing multiple concurrent RCA sessions

### 2. Updated RCA Trigger Handler (`internal/handlers/rca_trigger.go`)
Modified the payload construction to populate the `SessionID` field:

```go
payload := models.RCAGraphPayload{
    RCAContext: models.RCAContext{
        AlertID:      alert.AlertId,
        RootEntityID: rootEntityID,
        SessionID:    alert.AlertId, // Use incident ID as sessionId for Redis context in n8n
        GeneratedAt:  time.Now(),
    },
    Nodes: nodes,
    Edges: edges,
}
```

**Key Points:**
- The incident ID (`alert.AlertId`) is used as the sessionId
- This ensures each incident has its own isolated conversation context in Redis
- The sessionId is included in every RCA trigger request to n8n

## How It Works

### Flow:
1. **User triggers RCA** for an alert via `POST /api/v1/alerts/:id/rca/trigger`
2. **Backend builds the RCA graph** with alert, entity, change, and topology data
3. **SessionID is injected** into the payload using the incident's alert ID
4. **Payload is sent to n8n** (when the actual webhook call is implemented)
5. **n8n workflow receives the payload** with the sessionId
6. **n8n uses sessionId** to:
   - Store conversation context in Redis (e.g., `SET rca:session:{sessionId} {context}`)
   - Retrieve context for follow-up questions (e.g., `GET rca:session:{sessionId}`)
   - Maintain state across multiple LLM interactions for the same incident

### Example Payload:
```json
{
  "rca_context": {
    "alert_id": "INC-2026-001",
    "root_entity_id": "db-prod-01",
    "sessionId": "INC-2026-001",
    "generated_at": "2026-02-01T08:30:00Z"
  },
  "nodes": [...],
  "edges": [...]
}
```

## n8n Workflow Integration

The n8n workflow should use the `sessionId` field as follows:

### Setting Context:
```javascript
// In n8n workflow node
const sessionId = $json.rca_context.sessionId;
const context = {
  alert_id: $json.rca_context.alert_id,
  root_entity: $json.rca_context.root_entity_id,
  graph_data: { nodes: $json.nodes, edges: $json.edges },
  conversation_history: []
};

// Store in Redis
await redis.set(`rca:session:${sessionId}`, JSON.stringify(context), 'EX', 3600);
```

### Retrieving Context:
```javascript
// In subsequent chatbot interactions
const sessionId = $json.sessionId; // From user's chat request
const context = await redis.get(`rca:session:${sessionId}`);
const parsedContext = JSON.parse(context);

// Use context for LLM prompt
const prompt = `
Given the following incident context:
Alert: ${parsedContext.alert_id}
Root Entity: ${parsedContext.root_entity}
Previous conversation: ${parsedContext.conversation_history.join('\n')}

User question: ${$json.message}
`;
```

## Benefits

1. **Session Isolation**: Each incident has its own conversation context
2. **Stateful Conversations**: Follow-up questions can reference previous context
3. **Scalability**: Redis provides fast, distributed session storage
4. **Debugging**: SessionId makes it easy to trace conversations in logs
5. **Cleanup**: Sessions can be set with TTL (e.g., 1 hour) for automatic cleanup

## Testing

To test the implementation:

1. **Trigger RCA for an alert:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/alerts/{alert_id}/rca/trigger \
     -H "Authorization: Bearer {token}"
   ```

2. **Verify the payload includes sessionId:**
   ```json
   {
     "status": "graph_generated",
     "payload_preview": {
       "rca_context": {
         "alert_id": "...",
         "sessionId": "..."  // Should match alert_id
       }
     }
   }
   ```

3. **In n8n workflow**, verify the sessionId is accessible:
   ```javascript
   console.log('SessionId:', $json.rca_context.sessionId);
   ```

## Next Steps

1. **Update frontend to use the proxy endpoint** instead of calling n8n directly:
   ```javascript
   // OLD: Direct call to n8n
   // fetch('http://192.168.1.201:5678/webhook/alert-chat', {...})
   
   // NEW: Call through backend proxy
   fetch('/api/v1/chatbot', {
     method: 'POST',
     headers: {
       'Authorization': `Bearer ${token}`,
       'Content-Type': 'application/json'
     },
     body: JSON.stringify({
       action: 'init',
       alert: alertData,
       graph_data: graphData
     })
   })
   ```

2. **Configure n8n workflow** to use the sessionId for Redis operations
3. **Implement session cleanup** logic (optional, can use Redis TTL)

## Chatbot Proxy Implementation

### Problem
The frontend chatbot was calling n8n directly without a sessionId, preventing Redis-based context management.

### Solution
Created a **backend proxy** (`/api/v1/chatbot`) that:
1. Receives the chatbot request from the frontend
2. Extracts the incident ID from `alert.alertid`
3. Injects it as `sessionId` in the payload
4. **Enriches payload with RCA Graph Data** (if missing):
   - Fetches the full alert details from DB
   - Generates the **Entity Topology Graph** (Nodes with Alert/Change details + Edges)
   - Note: Uses `BuildEntityGraph` to ensure `nodes` contain `alerts`, `changes`, `support_owner` lists.
   - Injects it into `graph_data` field (and optionally `context.graph_data`)
5. Forwards the modified request to n8n
6. Returns n8n's response to the frontend

### Benefits
- ✅ **No frontend changes required** (just change the endpoint URL)
- ✅ **Centralized sessionId logic** in the backend
- ✅ **Automatic Context Enrichment**: Chatbot always receives full graph data even if frontend doesn't send it
- ✅ **Detailed Topology**: Graph nodes include associated alerts and changes for better RCA context
- ✅ **Additional processing/validation** can be added easily
- ✅ **Consistent sessionId format** across all RCA interactions

### Endpoints Created

#### 1. `POST /api/v1/chatbot`
Standard chatbot requests (init, follow-up questions)

**Request:**
```json
{
  "action": "init",
  "alert": {
    "alertid": "INC-2026-001",
    ...
  }
  // graph_data is missing!
}
```

**Modified payload sent to n8n:**
```json
{
  "action": "init",
  "sessionId": "INC-2026-001",
  "alert": {
      "alertid": "INC-2026-001",
      ...
      "graph_data": {  // ← Injected INSIDE alert
         "root": "...",
         "nodes": [
            { "name": "...", "alerts": [...], "changes": [...] }
         ],
         "edges": [...]
      }
  }
}
```

#### 2. `POST /api/v1/chatbot/stream`
Streaming chatbot responses (SSE/Server-Sent Events)

Same sessionId injection logic, but handles streaming responses.

### Configuration

Set the n8n webhook URL via environment variable:
```bash
export N8N_CHAT_WEBHOOK_URL="http://192.168.1.201:5678/webhook/alert-chat"
```

Or it will default to: `http://192.168.1.201:5678/webhook/alert-chat`

## Files Modified

- `internal/models/rca_graph.go` - Added SessionID field to RCAContext
- `internal/handlers/rca_trigger.go` - Populated SessionID with incident ID
- `internal/handlers/chatbot.go` - **NEW** Chatbot proxy with sessionId injection
- `cmd/main.go` - Added chatbot proxy routes

## Build Status

✅ Code compiles successfully with no errors
