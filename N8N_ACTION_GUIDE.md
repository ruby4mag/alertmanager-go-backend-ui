# n8n Workflow Configuration for Interactive Actions

To enable buttons and forms in your Chatbot, your n8n workflow must return a specific JSON structure containing an `actions` array.

## Workflow Logic
You simply need to "attach" the actions to your final response before sending it back to the webhook.

**Flow:** `[Webhook] -> [AI Processing] -> [Code Node] -> [Respond to Webhook]`

---

## 🏗️ Step 1: Add a "Code Node"
Place a **Code Node** just before your final "Respond to Webhook" node. This node will construct the JSON payload.

**Paste this JavaScript into the Code Node:**

```javascript
// 1. Get the text response from your previous AI/LLM node
// specific field depends on your previous node (e.g. 'text', 'output', 'content')
const aiMessage = $('AI Agent').first().json.output || "Analysis complete.";

// 2. Define the Actions (Buttons/Forms)
// You can make this dynamic based on the AI response if needed
const interactiveActions = [
  {
    "type": "button",
    "label": "🚨 Create Incident",
    "action_id": "init_pd_creation",
    "style": "danger"
  },
  {
    "type": "button",
    "label": "✅ Resolve Alert",
    "action_id": "resolve_alert",
    "style": "success"
  }
];

// 3. Output the final JSON structure for the Frontend
return {
  "output": aiMessage,
  "actions": interactiveActions
};
```

*(Note: Adjust `$('AI Agent')` to match the name of your actual previous node)*

---

## 📡 Step 2: Configure "Respond to Webhook" Node
This node sends the data back to the Go Backend (and then to the Frontend).

1.  **Respond With**: `JSON`
2.  **Response Data**: `First Item JSON` (Important! This removes the `[...]` array wrapper so frontend gets a clean object)
3.  **Response Key**: (Leave empty)
    *   Ensure the output JSON looks like:
        ```json
        {
          "output": "Your text here...",
          "actions": [ ... ]
        }
        ```

---

## 🔁 Handling the Button Click (The Loop)
When a user clicks a button, n8n will receive a **NEW** webhook call with:
`"action": "init_pd_creation"`

You must handle this in your workflow (e.g., using a **Switch Node** after the Webhook):

1.  **Switch Node**: Filter on `{{ $json.body.action }}`
    *   If `action == 'init'`: Run Standard Analysis -> Code Node (Show Buttons) -> Respond.
    *   If `action == 'init_pd_creation'`: Run PagerDuty Node -> Code Node (Say "Created!") -> Respond.

This creates the interactive loop.
