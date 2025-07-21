package embed

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"
)

const endpoint = "https://api.openai.com/v1/embeddings"
const modelID  = "text-embedding-3-small"

type openAIReq struct {
	Input string `json:"input"`
	Model string `json:"model"`
}
type openAIResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Vector returns a 1536-dim float32 embedding for the given text.
func Vector(text string) ([]float32, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, errors.New("OPENAI_API_KEY not set")
	}

	body, _ := json.Marshal(openAIReq{Input: text, Model: modelID})
	req, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	c := http.Client{Timeout: 20 * time.Second}
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("openai: " + res.Status)
	}

	var out openAIResp
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Data[0].Embedding, nil
}