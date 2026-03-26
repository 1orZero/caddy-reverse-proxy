package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateProxyMuxRoutesDoubaoNamespace(t *testing.T) {
	var receivedPath string
	var receivedAuth string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	mux, err := createProxyMux(upstream.URL)
	if err != nil {
		t.Fatalf("createProxyMux returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/doubao/v1/audio/transcriptions?format=json", nil)
	request.Header.Set("Authorization", "Bearer doubao-key")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204 response, got %d", response.Code)
	}

	if receivedPath != "/v1/audio/transcriptions" {
		t.Fatalf("expected Doubao upstream path /v1/audio/transcriptions, got %s", receivedPath)
	}

	if receivedAuth != "Bearer doubao-key" {
		t.Fatalf("expected Authorization header to be forwarded, got %s", receivedAuth)
	}
}

func TestCreateProxyMuxRoutesOpenRouterNamespace(t *testing.T) {
	var receivedPath string
	var receivedAuth string

	openRouter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer openRouter.Close()

	previousTarget := openRouterTarget
	openRouterTarget = openRouter.URL
	defer func() {
		openRouterTarget = previousTarget
	}()

	mux, err := createProxyMux("https://example.com")
	if err != nil {
		t.Fatalf("createProxyMux returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/openrouter/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer openrouter-key")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202 response, got %d", response.Code)
	}

	if receivedPath != "/api/v1/chat/completions" {
		t.Fatalf("expected OpenRouter upstream path /api/v1/chat/completions, got %s", receivedPath)
	}

	if receivedAuth != "Bearer openrouter-key" {
		t.Fatalf("expected Authorization header to be forwarded, got %s", receivedAuth)
	}
}

func TestCreateProxyMuxReturns404ForUnknownPath(t *testing.T) {
	mux, err := createProxyMux("https://example.com")
	if err != nil {
		t.Fatalf("createProxyMux returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404 response, got %d", response.Code)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if string(body) != "404 page not found\n" {
		t.Fatalf("expected default 404 body, got %q", string(body))
	}
}
