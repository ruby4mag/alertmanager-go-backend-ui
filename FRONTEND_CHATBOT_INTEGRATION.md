# Frontend Integration Guide: Chatbot with SessionId

## Quick Start

### Change Required
Update your chatbot to call the backend proxy instead of n8n directly.

### Before (Direct n8n call):
```javascript
const response = await fetch('http://192.168.1.201:5678/webhook/alert-chat', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    action: 'init',
    alert: alertData,
    graph_data: graphData
  })
});
```

### After (Backend proxy):
```javascript
const response = await fetch('/api/v1/chatbot', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${authToken}`,  // ← Add auth token
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    action: 'init',
    alert: alertData,
    graph_data: graphData
  })
});
```

## What Changed?

1. **Endpoint URL**: `http://192.168.1.201:5678/webhook/alert-chat` → `/api/v1/chatbot`
2. **Authentication**: Added `Authorization` header (required for protected routes)
3. **SessionId**: Automatically injected by the backend (no frontend changes needed)

## Benefits

- ✅ **Automatic sessionId injection** - Backend extracts `alert.alertid` and adds it as `sessionId`
- ✅ **Redis context management** - n8n can now maintain conversation context per incident
- ✅ **Centralized logic** - All sessionId handling is in one place
- ✅ **Better security** - Requests are authenticated through the backend

## API Endpoints

### 1. Standard Chatbot Request
**Endpoint:** `POST /api/v1/chatbot`

**Use for:**
- Initial chatbot conversation (`action: "init"`)
- Follow-up questions
- Any non-streaming chatbot interaction

**Example:**
```javascript
const response = await fetch('/api/v1/chatbot', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    action: 'init',  // or 'message' for follow-ups
    alert: {
      alertid: 'INC-2026-001',
      entity: 'db-prod-01',
      // ... other alert fields
    },
    graph_data: {
      nodes: [...],
      edges: [...]
    }
  })
});

const data = await response.json();
```

### 2. Streaming Chatbot Request
**Endpoint:** `POST /api/v1/chatbot/stream`

**Use for:**
- Server-Sent Events (SSE)
- Streaming AI responses
- Real-time chatbot updates

**Example:**
```javascript
const response = await fetch('/api/v1/chatbot/stream', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    action: 'message',
    sessionId: 'INC-2026-001',  // Optional: can be auto-extracted from alert
    message: 'What caused this incident?'
  })
});

// Handle streaming response
const reader = response.body.getReader();
const decoder = new TextDecoder();

while (true) {
  const { done, value } = await reader.read();
  if (done) break;
  
  const chunk = decoder.decode(value);
  console.log('Received chunk:', chunk);
  // Update UI with streaming response
}
```

## Request Payload Structure

### Initial Conversation (action: "init")
```json
{
  "action": "init",
  "alert": {
    "alertid": "INC-2026-001",
    "entity": "db-prod-01",
    "alertsummary": "Database connection timeout",
    "severity": "CRITICAL",
    // ... other alert fields
  },
  "graph_data": {
    "nodes": [
      {
        "name": "db-prod-01",
        "has_alert": true,
        "severity": "CRITICAL",
        // ...
      }
    ],
    "edges": [...]
  }
}
```

### Follow-up Question
```json
{
  "action": "message",
  "sessionId": "INC-2026-001",  // Optional if alert is included
  "message": "What are the related changes?",
  "alert": {
    "alertid": "INC-2026-001"
    // ... minimal alert info for sessionId extraction
  }
}
```

## Backend Processing

When you call `/api/v1/chatbot`, the backend:

1. **Receives** your request
2. **Extracts** the incident ID from `alert.alertid`
3. **Injects** `sessionId: "INC-2026-001"` into the payload
4. **Forwards** to n8n at `http://192.168.1.201:5678/webhook/alert-chat`
5. **Returns** n8n's response to you

### What n8n Receives:
```json
{
  "action": "init",
  "sessionId": "INC-2026-001",  // ← Automatically added
  "alert": {
    "alertid": "INC-2026-001",
    // ...
  },
  "graph_data": {...}
}
```

## Error Handling

```javascript
try {
  const response = await fetch('/api/v1/chatbot', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(payload)
  });

  if (!response.ok) {
    const error = await response.json();
    console.error('Chatbot error:', error);
    // Handle error (show message to user)
    return;
  }

  const data = await response.json();
  // Process successful response
  
} catch (error) {
  console.error('Network error:', error);
  // Handle network error
}
```

## Common Issues

### 1. Missing Authorization Header
**Error:** `401 Unauthorized`
**Fix:** Add the `Authorization: Bearer ${token}` header

### 2. Invalid Alert ID
**Warning:** `"Warning: Could not extract alert ID for sessionId"`
**Fix:** Ensure `alert.alertid` is present in your payload

### 3. CORS Issues
**Error:** CORS policy error
**Fix:** Backend already has CORS configured for `http://192.168.1.201:3000`

## Testing

### Test the endpoint:
```bash
curl -X POST http://localhost:8080/api/v1/chatbot \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "init",
    "alert": {
      "alertid": "TEST-001",
      "entity": "test-entity"
    },
    "graph_data": {
      "nodes": [],
      "edges": []
    }
  }'
```

### Check backend logs:
```
Injected sessionId: TEST-001 into chatbot request
Chatbot request forwarded to n8n with sessionId: TEST-001, status: 200
```

## Migration Checklist

- [ ] Update chatbot endpoint URL to `/api/v1/chatbot`
- [ ] Add `Authorization` header with bearer token
- [ ] Test initial conversation (`action: "init"`)
- [ ] Test follow-up questions
- [ ] Verify sessionId appears in n8n webhook logs
- [ ] Test streaming endpoint if using SSE
- [ ] Update error handling for new endpoint
- [ ] Remove any hardcoded n8n URLs from frontend

## Questions?

If you encounter any issues:
1. Check browser console for errors
2. Check backend logs for sessionId injection messages
3. Verify n8n is receiving the sessionId field
4. Ensure Redis is configured in n8n workflow to use `sessionId`
