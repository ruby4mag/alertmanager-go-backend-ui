package ai

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

// QdrantConfig holds connection details
type QdrantConfig struct {
    URL string
}

// OllamaConfig holds connection details
type OllamaConfig struct {
    URL   string
    Model string
}

var (
    QdrantStub QdrantConfig
    OllamaStub OllamaConfig
)

func InitAI(qdrantURL, ollamaURL, ollamaModel string) {
    QdrantStub = QdrantConfig{URL: qdrantURL}
    OllamaStub = OllamaConfig{URL: ollamaURL, Model: ollamaModel}
}

// ---------------------------------------------------------------------------
// OLLAMA CLIENT
// ---------------------------------------------------------------------------

type EmbeddingRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
}

type EmbeddingResponse struct {
    Embedding []float64 `json:"embedding"`
}

func GetEmbedding(text string) ([]float64, error) {
    url := fmt.Sprintf("%s/api/embeddings", OllamaStub.URL)
    payload := EmbeddingRequest{
        Model:  OllamaStub.Model,
        Prompt: text,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }

    resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(b))
    }

    var result EmbeddingResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result.Embedding, nil
}

// ---------------------------------------------------------------------------
// QDRANT CLIENT (REST)
// ---------------------------------------------------------------------------

// UpsertPointReq matches Qdrant REST structure
type UpsertPointReq struct {
    Points []Point `json:"points"`
}

type Point struct {
    ID      string         `json:"id"`
    Vector  map[string][]float64 `json:"vector,omitempty"` // Named vectors
    Payload map[string]interface{} `json:"payload,omitempty"`
}

// EnsureCollection creates collection if not exists (simplified)
func EnsureCollection(name string, vectorSize int) error {
    url := fmt.Sprintf("%s/collections/%s", QdrantStub.URL, name)
    
    // Check if exists
    resp, err := http.Get(url)
    if err == nil && resp.StatusCode == 200 {
        return nil // exists
    }

    // Create
    createBody := map[string]interface{}{
        "vectors": map[string]interface{}{
            "rca_summary": map[string]interface{}{
                "size": vectorSize,
                "distance": "Cosine",
            },
            "alert_pattern": map[string]interface{}{
                "size": vectorSize,
                "distance": "Cosine",
            },
            "topology_pattern": map[string]interface{}{
                "size": vectorSize,
                "distance": "Cosine",
            },
        },
    }
    b, _ := json.Marshal(createBody)
    
    req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(b))
    req.Header.Set("Content-Type", "application/json")
    
    client := &http.Client{}
    resp, err = client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
         return fmt.Errorf("failed to create collection: %v", resp.Status)
    }
    return nil
}

func UpsertVectors(collection string, id string, vectors map[string][]float64, payload map[string]interface{}) error {
    url := fmt.Sprintf("%s/collections/%s/points?wait=true", QdrantStub.URL, collection)

    point := Point{
        ID:      id,
        Vector:  vectors,
        Payload: payload,
    }

    reqBody := UpsertPointReq{
        Points: []Point{point},
    }

    b, err := json.Marshal(reqBody)
    if err != nil {
        return err
    }

    // Changed to PUT for explicit Upsert
    req, err := http.NewRequest("PUT", url, bytes.NewBuffer(b))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        content, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("qdrant upsert failed: %s : %s", resp.Status, string(content))
    }

    return nil
}
