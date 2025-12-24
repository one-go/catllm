package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/one-go/catllm/internal/types"
)

// Codec handles OpenAI protocol conversion
type Codec struct{}

// Encode converts unified request to OpenAI format
func (c *Codec) Encode(req *types.UnifiedRequest) ([]byte, error) {
	return json.Marshal(req)
}

// Decode converts OpenAI response to unified format
func (c *Codec) Decode(body io.Reader) (*types.UnifiedResponse, error) {
	var resp types.UnifiedResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resp, nil
}

// DecodeError extracts error information from OpenAI error response
func (c *Codec) DecodeError(body io.Reader) error {
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(body).Decode(&errResp); err != nil {
		return fmt.Errorf("unknown error")
	}
	return fmt.Errorf("%s: %s", errResp.Error.Type, errResp.Error.Message)
}

// BuildRequest creates an HTTP request for OpenAI
func (c *Codec) BuildRequest(ctx context.Context, baseURL, apiKey, path string, payload []byte) (*http.Request, error) {
	url := baseURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	return req, nil
}
