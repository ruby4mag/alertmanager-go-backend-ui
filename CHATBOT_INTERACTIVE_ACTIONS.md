# Interactive Chatbot Actions: Implementation Guide

This guide details how to implement interactive elements (buttons, forms) in the Chatbot UI to support workflows like "Create PagerDuty Incident".

## 1. Overview
The backend/n8n workflow will now send **Structured Responses** that include not just text, but functionality definitions (Actions). The frontend needs to parse these and render clickable elements.

## 2. JSON Protocol

### A. Chatbot Response (Server -> Client)
The chatbot stream (or HTTP response) will include an `actions` array when user input is required.

**Example 1: Simple Action Button**
```json
{
  "output": "I've identified a critical issue with the database layer. I recommend declaring an incident.",
  "actions": [
    {
      "type": "button",
      "label": "🚨 Create PagerDuty Incident",
      "action_id": "init_pd_creation",
      "style": "danger" // primary, secondary, danger
    }
  ]
}
```

**Example 2: Selection/Form (Follow-up)**
After clicking the button above, the server might ask for details:
```json
{
  "output": "Please select the PagerDuty service to assign:",
  "actions": [
    {
      "type": "select",
      "key": "pd_service_id",
      "options": [
        { "label": "Platform Ops", "value": "P123456" },
        { "label": "Database Team", "value": "P987654" }
      ],
      "submit_label": "Confirm & Create" // The button to submit the selection
    }
  ]
}
```

---

## 3. Frontend Implementation Steps

### Step 1: Update Message Component
Modify the component that renders chat messages to check for the `actions` field.

**Important Note on Response Format:**
Depending on n8n configuration, the response might be an object `{...}` or an array `[{...}]`.
Always parse robustly:
```js
const responseData = Array.isArray(response) ? response[0] : response;
const actions = responseData.actions || [];
```

**Pseudo-Code (React-like):**
```jsx
function ChatMessage({ message }) {
  // Normalize message parsing ...
  return (
    <div className="message-container">
      {/* 1. Render Text Content (Markdown) */}
      <Markdown>{message.output}</Markdown>

      {/* 2. Render Actions if present */}
      {message.actions && (
        <div className="message-actions">
          {message.actions.map(action => (
            <ActionComponent 
              action={action} 
              onExecute={handleActionExecute} 
            />
          ))}
        </div>
      )}
    </div>
  );
}
```

### Step 2: Implement Action Components

**A. Button Renderer:**
If `action.type === 'button'`:
- Render a `<button>` element.
- Label: `action.label`.
- Style: Map `action.style` to CSS classes (e.g., `danger` -> `bg-red-500`).
- **On Click**: Call `handleActionExecute(action.action_id, {})`.

**B. Select/Form Renderer:**
If `action.type === 'select'`:
- Render a `<select>` dropdown using `action.options`.
- Manage local state for the selected value.
- Render a "Submit" button (`action.submit_label`).
- **On Submit**: Call `handleActionExecute('submit_selection', { [action.key]: selectedValue })`.

---

## 4. API Interaction (Client -> Server)

When an action is triggered, send a POST request to the **Chatbot Proxy** (`/api/v1/chatbot`).

**Endpoint**: `POST /api/v1/chatbot`

**Payload Construction:**
You must preserve the `sessionId` and current Alert context.

**Scenario A: Clicking a Simple Button**
*User clicks "Create PagerDuty Incident" (`action_id: "init_pd_creation"`)*

```json
{
  "action": "init_pd_creation",      // <--- The action_id from the button matches this
  "sessionId": "INC-2026-001",       // <--- REQUIRED: Maintain session
  "alert": {                         // <--- OPTIONAL: Include if context is needed
     "alertid": "INC-2026-001"
  }
}
```

**Scenario B: Submitting a Selection**
*User selects "Platform Ops" and clicks Confirm*

```json
{
  "action": "execute_pd_creation",   // <--- Defined by your internal logic or previous step
  "sessionId": "INC-2026-001",
  "payload": {                       // <--- Send form data here
     "pd_service_id": "P123456"
  }
}
```

## 5. Handling the Response
The response to these action requests will be a standard chatbot response stream.
1. Clear the previous actions (prevent double-clicking).
2. Append the new User Message (e.g., "Clicked: Create Incident") to the chat history for visual continuity.
3. Stream the new AI response.

---

## Summary Checklist for Frontend Dev
- [ ] Parse `actions` array in message objects.
- [ ] Create UI components for `button` and `select` types.
- [ ] Update API service to send `action` field dynamically based on user interaction (instead of hardcoded "init").
- [ ] Ensure `sessionId` is passed in every request.
