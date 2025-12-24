// Package server implements the HTTP server for handling requests.
package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/one-go/catllm/internal/codec/openai"
	"github.com/one-go/catllm/internal/config"
	"github.com/one-go/catllm/internal/forwarder"
	"github.com/one-go/catllm/internal/types"
)

// Server handles HTTP requests
type Server struct {
	config    *config.Config
	forwarder *forwarder.Forwarder
	codec     *openai.Codec
}

// New creates a new server
func New(cfg *config.Config) *Server {
	return &Server{
		config:    cfg,
		forwarder: forwarder.New(30 * time.Second),
		codec:     &openai.Codec{},
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	http.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	http.HandleFunc("/responses", s.handleResponses)
	http.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf(":%d", s.config.Server.Port)
	slog.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var req types.UnifiedRequest
	requestBody, err := decodeUnifiedRequest(r, &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}
	slog.Debug("request received", "path", r.URL.Path, "method", r.Method, "body", string(requestBody))

	providerName := s.config.GetRoute(req.Model)
	if providerName == "" {
		http.Error(w, fmt.Sprintf("no route for model: %s", req.Model), http.StatusNotFound)
		return
	}

	provider := s.config.GetProvider(providerName)
	if provider == nil {
		http.Error(w, fmt.Sprintf("provider not found: %s", providerName), http.StatusInternalServerError)
		return
	}

	payload, err := s.codec.Encode(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("encode error: %v", err), http.StatusInternalServerError)
		return
	}
	slog.Debug("upstream request", "provider", providerName, "base_url", provider.BaseURL, "path", "/v1/chat/completions", "body", string(payload))

	upstreamReq, err := s.codec.BuildRequest(r.Context(), provider.BaseURL, provider.APIKey, "/v1/chat/completions", payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("build request error: %v", err), http.StatusInternalServerError)
		return
	}

	resp, err := s.forwarder.Do(r.Context(), upstreamReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("forward error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	slog.Debug("upstream response", "provider", providerName, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		err := s.codec.DecodeError(resp.Body)
		http.Error(w, err.Error(), resp.StatusCode)
		return
	}

	if req.Stream {
		slog.Debug("streaming response start", "provider", providerName)
		bytesWritten, err := s.handleStreamingResponse(w, resp)
		if err != nil {
			slog.Error("streaming response error", "provider", providerName, "err", err)
			return
		}
		slog.Debug("streaming response complete", "provider", providerName, "bytes", bytesWritten)
		return
	}

	upstreamBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read upstream response: %v", err), http.StatusBadGateway)
		return
	}
	slog.Debug("upstream response body", "provider", providerName, "status", resp.StatusCode, "body", string(upstreamBody))

	unifiedResp, err := s.codec.Decode(bytes.NewReader(upstreamBody))
	if err != nil {
		http.Error(w, fmt.Sprintf("decode error: %v", err), http.StatusInternalServerError)
		return
	}

	responseBody, err := json.Marshal(unifiedResp)
	if err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
		return
	}
	slog.Debug("response body", "path", r.URL.Path, "body", string(responseBody))

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(responseBody)
}

func (s *Server) handleStreamingResponse(w http.ResponseWriter, resp *http.Response) (int64, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return 0, fmt.Errorf("streaming not supported")
	}

	buf := make([]byte, 4096)
	var total int64
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			total += int64(n)
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return total, writeErr
			}
			flusher.Flush()
		}
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}
	}
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var req types.UnifiedRequest
	requestBody, err := decodeUnifiedRequest(r, &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}
	slog.Debug("request received", "path", r.URL.Path, "method", r.Method, "body", string(requestBody))

	providerName := s.config.GetRoute(req.Model)
	if providerName == "" {
		http.Error(w, fmt.Sprintf("no route for model: %s", req.Model), http.StatusNotFound)
		return
	}

	provider := s.config.GetProvider(providerName)
	if provider == nil {
		http.Error(w, fmt.Sprintf("provider not found: %s", providerName), http.StatusInternalServerError)
		return
	}

	payload, err := s.codec.Encode(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("encode error: %v", err), http.StatusInternalServerError)
		return
	}
	slog.Debug("upstream request", "provider", providerName, "base_url", provider.BaseURL, "path", "/responses", "body", string(payload))

	upstreamReq, err := s.codec.BuildRequest(r.Context(), provider.BaseURL, provider.APIKey, "/responses", payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("build request error: %v", err), http.StatusInternalServerError)
		return
	}

	resp, err := s.forwarder.Do(r.Context(), upstreamReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("forward error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	slog.Debug("upstream response", "provider", providerName, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		upstreamBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			http.Error(w, fmt.Sprintf("read upstream error: %v", readErr), http.StatusBadGateway)
			return
		}
		slog.Debug("upstream error body", "provider", providerName, "status", resp.StatusCode, "body", string(upstreamBody))
		err := s.codec.DecodeError(bytes.NewReader(upstreamBody))
		http.Error(w, err.Error(), resp.StatusCode)
		return
	}

	if req.Stream {
		slog.Debug("streaming response start", "provider", providerName)
		bytesWritten, err := s.handleStreamingResponse(w, resp)
		if err != nil {
			slog.Error("streaming response error", "provider", providerName, "err", err)
			return
		}
		slog.Debug("streaming response complete", "provider", providerName, "bytes", bytesWritten)
		return
	}

	upstreamBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read upstream response: %v", err), http.StatusBadGateway)
		return
	}
	slog.Debug("upstream response body", "provider", providerName, "status", resp.StatusCode, "body", string(upstreamBody))

	unifiedResp, err := s.codec.Decode(bytes.NewReader(upstreamBody))
	if err != nil {
		http.Error(w, fmt.Sprintf("decode error: %v", err), http.StatusInternalServerError)
		return
	}

	responseBody, err := json.Marshal(unifiedResp)
	if err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
		return
	}
	slog.Debug("response body", "path", r.URL.Path, "body", string(responseBody))

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(responseBody)
}

func decodeUnifiedRequest(r *http.Request, dst *types.UnifiedRequest) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return body, err
	}
	return body, nil
}
