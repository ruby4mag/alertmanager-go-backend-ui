curl -X POST http://localhost:8081/api/v1/changes \
  -H "Content-Type: application/json" \
  -d '{
    "change_id": "CHG-2024-001",
    "source": "servicenow",
    "change_type": "deployment",
    "name": "UPS Power Upgradation",
    "description": "UPS maintenance",
    "status": "in_progress",
    "start_time": "2026-02-01T10:46:09Z",
    "end_time": "2026-02-01T12:46:09Z",
    "implemented_by": "DC Operations Team",
    "affected_entities": [
      "DC-1"
    ],
    "raw_payload": {
      "ticket_number": "INC12345",
      "approver": "Jane Doe",
      "environment": "production"
    }
  }'