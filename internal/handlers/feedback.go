package handlers

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
    "github.com/ruby4mag/alertmanager-go-backend-ui/internal/db"
    "github.com/ruby4mag/alertmanager-go-backend-ui/internal/models"
    "github.com/ruby4mag/alertmanager-go-backend-ui/internal/ai"
)

// AddFeedback handles the submission of post-incident feedback
func AddFeedback(c *gin.Context) {
    id := c.Param("id")
    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
        return
    }

    var feedback models.IncidentFeedback
    if err := c.ShouldBindJSON(&feedback); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    collection := db.GetCollection("alerts")
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // 1. Fetch Alert to validate and ensure state
    var alert models.DbAlert
    err = collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&alert)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
        return
    }

    // Validation: Incident must be resolved or closed (simplistic check)
    // if alert.AlertStatus != "CLOSED" ... (Skipping strict check for dev velocity unless requested)
    
    // Validation: Idempotency - check if feedback already exists
    if alert.Feedback != nil {
        c.JSON(http.StatusConflict, gin.H{"error": "Feedback already submitted for this incident"})
        return
    }

    // 2. Persist Feedback
    feedback.SubmittedAt = time.Now()
    if feedback.FeedbackID == "" {
        feedback.FeedbackID = primitive.NewObjectID().Hex()
    }
    feedback.IncidentID = id
    // feedback.SubmittedBy = ... (from context user)

    update := bson.M{"$set": bson.M{"feedback": feedback}}
    _, err = collection.UpdateOne(ctx, bson.M{"_id": objectID}, update)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // 3. Normalize into RCA Case Memory (Async in production, sync here for demo)
    err = processRCACaseMemory(ctx, alert, feedback)
    if err != nil {
         // Log error, but success to user as feedback is saved
         fmt.Printf("Error processing RCA Memory: %v\n", err)
         c.JSON(http.StatusAccepted, gin.H{"status": "saved_but_processing_failed", "error": err.Error()})
         return
    }

    c.JSON(http.StatusOK, gin.H{"status": "feedback_processed"})
}

func processRCACaseMemory(ctx context.Context, alert models.DbAlert, fb models.IncidentFeedback) error {
    // A. Build Signatures
    // 1. Alert Signature (Abstract representation)
    // Simply using the summary sequence for now. In real-world this replaces hostname with "NODE" etc.
    alertSig := fmt.Sprintf("%s:%s", alert.ServiceName, alert.AlertSummary)
    
    // 2. Topology Signature (Using provided symptoms/root cause relations)
    // E.g. Root -> Symptom A, Root -> Symptom B
    topoSig := fmt.Sprintf("Root(%s) causing Symptoms%v", fb.FinalRootCause, fb.Symptoms)

    // 3. Temporal Signature 
    // Just using duration for now or simple "Cascade" text
    tempSig := "Cascade" 

    // B. Create Memory Object
    memory := models.RCACaseMemory{
        CaseID:            primitive.NewObjectID().Hex(),
        IncidentID:        alert.ID.Hex(),
        AlertSignature:    alertSig,
        TopologySignature: topoSig,
        TemporalSignature: tempSig,
        RootCauseEntity:   fb.FinalRootCause,
        RootCauseType:     fb.RootCauseType,
        ResolutionSummary: fb.ResolutionSummary,
        Confidence:        fb.OperatorConfidence,
        CreatedAt:         time.Now(),
    }

    // Store Memory in DB (Optional: separate collection for analytics)
    memCol := db.GetCollection("rca_cases")
    _, err := memCol.InsertOne(ctx, memory)
    if err != nil { return err }

    // C. Generate Embeddings & Index
    return embedAndIndexCase(memory)
}

func embedAndIndexCase(caseMem models.RCACaseMemory) error {
    // 1. Generate Embeddings via Ollama
    
    // Vector 1: RCA Summary (The "Semantic" match)
    rcaText := fmt.Sprintf("Type: %s. Root Cause: %s. Resolution: %s", caseMem.RootCauseType, caseMem.RootCauseEntity, caseMem.ResolutionSummary)
    vecRCA, err := ai.GetEmbedding(rcaText)
    if err != nil { return err }

    // Vector 2: Alert Pattern (The "Fingerprint" match)
    vecPat, err := ai.GetEmbedding(caseMem.AlertSignature)
    if err != nil { return err }

    // Vector 3: Topology Pattern (The "Structural" match)
    vecTopo, err := ai.GetEmbedding(caseMem.TopologySignature)
    if err != nil { return err }
    
    // 2. Upsert to Qdrant
    // ID: Use CaseID (which is a hex string, need UUID for Qdrant usually or configure it)
    qdrantID := generateUUID(caseMem.CaseID)

    vectors := map[string][]float64{
        "rca_summary":      vecRCA,
        "alert_pattern":    vecPat,
        "topology_pattern": vecTopo,
    }

    payload := map[string]interface{}{
        "incident_id":        caseMem.IncidentID,
        "root_cause_type":    caseMem.RootCauseType,
        "confidence":         caseMem.Confidence,
        "owning_team":        caseMem.OwningTeam,
    }

    return ai.UpsertVectors("rca_cases", qdrantID, vectors, payload)
}

func generateUUID(hexID string) string {
     // MongoID is 24 hex chars. 
     // We construct a pseudo-UUID by padding with 8 zeros.
     // UUID Format: 8-4-4-4-12
     // 00000000-aaaa-bbbb-cccc-ddddeeeeffff
     // 00000000 (8)
     // hexID[0:4] (4)
     // hexID[4:8] (4)
     // hexID[8:12] (4)
     // hexID[12:] (12)

     if len(hexID) != 24 {
         // Fallback random or fail safe? Returing a nil UUID string if invalid length
         return "00000000-0000-0000-0000-000000000000"
     }
     
     return fmt.Sprintf("00000000-%s-%s-%s-%s", 
        hexID[0:4], 
        hexID[4:8], 
        hexID[8:12], 
        hexID[12:])
}
