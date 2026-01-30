# PagerDuty Integration - Backend Implementation Summary

## Overview
This document summarizes the backend implementation for PagerDuty integration in the AlertNinja application.

## Implementation Details

### 1. Database Schema
The implementation uses existing MongoDB collections:
- **Collection:** `pagerduty_services`
  - Fields: `service_id`, `service_name`
- **Collection:** `pagerduty_escalation_policies`
  - Fields: `ep_id`, `ep_name`

### 2. Updated Notify Rules Schema
The `notifyrules` collection now supports two additional optional fields:
- `pagerduty_service` (string, optional): ID of the PagerDuty service
- `pagerduty_escalation_policy` (string, optional): ID of the PagerDuty escalation policy

### 3. New API Endpoints

#### Get PagerDuty Services
**Endpoint:** `GET /api/pagerduty/services`

**Description:** Returns a list of all available PagerDuty services from MongoDB.

**Request:**
```http
GET /api/pagerduty/services
Authorization: Bearer <token>
```

**Response:**
```json
[
  {
    "id": "PJV72JW",
    "name": "SN: Jira Application",
    "description": ""
  }
]
```

**Status Codes:**
- `200 OK`: Successfully retrieved services
- `401 Unauthorized`: Invalid or missing authentication token
- `500 Internal Server Error`: Server error

---

#### Get PagerDuty Escalation Policies
**Endpoint:** `GET /api/pagerduty/escalation-policies`

**Description:** Returns a list of all available PagerDuty escalation policies from MongoDB.

**Request:**
```http
GET /api/pagerduty/escalation-policies
Authorization: Bearer <token>
```

**Response:**
```json
[
  {
    "id": "P02SNEP",
    "name": "SN:IT-Apps-MVP-ITES",
    "description": ""
  }
]
```

**Status Codes:**
- `200 OK`: Successfully retrieved escalation policies
- `401 Unauthorized`: Invalid or missing authentication token
- `500 Internal Server Error`: Server error

---

### 4. Updated Notify Rule Endpoints

The existing notify rule endpoints now support PagerDuty fields:

#### Create Notify Rule
**Endpoint:** `POST /api/notifyrules`

**Request Body:**
```json
{
  "rulename": "Critical Production Alerts",
  "ruledescription": "Notify on critical production issues",
  "ruleobject": "(Entity = 'production-api' and Severity = 'critical')",
  "payload": "{\"message\": \"Alert triggered\"}",
  "endpoint": "https://webhook.example.com/notify",
  "pagerduty_service": "PJV72JW",
  "pagerduty_escalation_policy": "P02SNEP"
}
```

#### Update Notify Rule
**Endpoint:** `PUT /api/notifyrules/{id}`

**Request Body:** Same as Create

#### Get Notify Rule
**Endpoint:** `GET /api/notifyrules/{id}`

**Response:**
```json
{
  "id": "123",
  "rulename": "Critical Production Alerts",
  "ruledescription": "Notify on critical production issues",
  "ruleobject": "{\"combinator\":\"and\",\"rules\":[{\"field\":\"Entity\",\"operator\":\"=\",\"value\":\"production-api\"}]}",
  "payload": "{\"message\": \"Alert triggered\"}",
  "endpoint": "https://webhook.example.com/notify",
  "pagerduty_service": "PJV72JW",
  "pagerduty_escalation_policy": "P02SNEP"
}
```

---

## Files Modified/Created

### Created Files:
1. **`/opt/alertninja/alertmanager-go-backend-ui/internal/models/pagerduty.go`**
   - Defines data models for PagerDuty services and escalation policies
   - Includes both database models and API response models

2. **`/opt/alertninja/alertmanager-go-backend-ui/internal/handlers/pagerduty.go`**
   - Implements handlers for fetching PagerDuty services and escalation policies
   - Transforms MongoDB documents to API response format

### Modified Files:
1. **`/opt/alertninja/alertmanager-go-backend-ui/internal/models/notifyrule.go`**
   - Added `PagerDutyService` field (optional)
   - Added `PagerDutyEscalationPolicy` field (optional)

2. **`/opt/alertninja/alertmanager-go-backend-ui/cmd/main.go`**
   - Added routes for PagerDuty endpoints:
     - `GET /api/pagerduty/services`
     - `GET /api/pagerduty/escalation-policies`

---

## Code Structure

### Models (`internal/models/pagerduty.go`)
```go
type DbPagerDutyService struct {
    ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    ServiceID   string             `bson:"service_id" json:"service_id"`
    ServiceName string             `bson:"service_name" json:"service_name"`
}

type DbPagerDutyEscalationPolicy struct {
    ID     primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    EpID   string             `bson:"ep_id" json:"ep_id"`
    EpName string             `bson:"ep_name" json:"ep_name"`
}

type PagerDutyServiceResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
}

type PagerDutyEscalationPolicyResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
}
```

### Handlers (`internal/handlers/pagerduty.go`)
- `GetPagerDutyServices(c *gin.Context)`: Fetches all services from MongoDB
- `GetPagerDutyEscalationPolicies(c *gin.Context)`: Fetches all escalation policies from MongoDB

Both handlers:
- Query MongoDB collections
- Transform database documents to API response format
- Return empty arrays instead of null when no data is found
- Handle errors appropriately

---

## Testing the Implementation

### 1. Start the Server
```bash
cd /opt/alertninja/alertmanager-go-backend-ui
go run cmd/main.go
```

### 2. Test PagerDuty Services Endpoint
```bash
curl -X GET \
  'http://localhost:8080/api/pagerduty/services' \
  -H 'Authorization: Bearer <your-token>'
```

### 3. Test PagerDuty Escalation Policies Endpoint
```bash
curl -X GET \
  'http://localhost:8080/api/pagerduty/escalation-policies' \
  -H 'Authorization: Bearer <your-token>'
```

### 4. Test Creating a Notify Rule with PagerDuty Fields
```bash
curl -X POST \
  'http://localhost:8080/api/notifyrules' \
  -H 'Authorization: Bearer <your-token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "rulename": "Test Rule",
    "ruledescription": "Test Description",
    "ruleobject": "test",
    "payload": "{}",
    "endpoint": "https://example.com",
    "pagerduty_service": "PJV72JW",
    "pagerduty_escalation_policy": "P02SNEP"
  }'
```

---

## Notes

1. **Authentication:** All endpoints are protected by the `AuthMiddleware()` and require a valid Bearer token.

2. **MongoDB Collections:** The implementation assumes that the MongoDB collections `pagerduty_services` and `pagerduty_escalation_policies` already exist and are populated with data.

3. **Optional Fields:** The PagerDuty fields in notify rules are optional. Existing notify rules without these fields will continue to work.

4. **Empty Arrays:** Both PagerDuty endpoints return empty arrays `[]` instead of `null` when no data is found, which is better for frontend consumption.

5. **Error Handling:** All endpoints include proper error handling and return appropriate HTTP status codes.

6. **Build Verification:** The implementation has been verified to compile successfully with `go build`.

---

## Frontend Integration

The frontend has already been updated to consume these endpoints:
- **New.js**: Fetches services and policies, sends values when creating rules
- **Edit.js**: Loads existing values, sends updates when saving
- **View.js**: Displays selected values in read-only mode

All frontend components use the following endpoints:
- `GET /api/pagerduty/services`
- `GET /api/pagerduty/escalation-policies`
- `POST /api/notifyrules` (with PagerDuty fields)
- `PUT /api/notifyrules/{id}` (with PagerDuty fields)
- `GET /api/notifyrules/{id}` (returns PagerDuty fields)

---

## Future Enhancements

1. **Validation:** Add validation to ensure selected service/policy IDs exist in the database
2. **Caching:** Implement caching for PagerDuty services and policies to reduce database queries
3. **Sync Job:** Create a background job to sync with PagerDuty API and update MongoDB collections
4. **Description Field:** Add description fields to MongoDB collections for richer UI display
5. **Pagination:** Add pagination support if the number of services/policies grows large
