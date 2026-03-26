package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

var openRouterTarget = "https://openrouter.ai"

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request details
		log.Printf("Received request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		// Check if request has a body and is JSON
		if r.Body != nil && r.ContentLength > 0 {
			contentType := r.Header.Get("Content-Type")
			if strings.Contains(strings.ToLower(contentType), "application/json") {
				// Read the body (limiting to 1MB to prevent memory issues)
				bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1048576))
				if err == nil {
					// Try to pretty print JSON
					var jsonData interface{}
					if json.Unmarshal(bodyBytes, &jsonData) == nil {
						prettyJSON, err := json.MarshalIndent(jsonData, "", "  ")
						if err == nil {
							log.Printf("Request body (JSON):\n%s", string(prettyJSON))
						} else {
							log.Printf("Request body (raw): %s", string(bodyBytes))
						}
					} else {
						log.Printf("Request body (raw): %s", string(bodyBytes))
					}

					// Replace the body so it can be read again by the proxy
					r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}
		}

		// Record the start time
		start := time.Now()

		// Create a custom response writer to capture the response status code
		responseWriter := &responseWriter{ResponseWriter: w}

		// Call the original handler
		handler.ServeHTTP(responseWriter, r)

		// Calculate the request duration
		duration := time.Since(start)

		// Log the response details
		log.Printf("Completed request: %s %s with status %d in %s", r.Method, r.URL.Path, responseWriter.statusCode, duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func createReverseProxy(target string, stripPrefix string, prependPrefix string) (http.Handler, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = rewriteProxyPath(targetURL.Path, req.URL.Path, stripPrefix, prependPrefix)
		req.URL.RawPath = req.URL.Path
		req.Host = req.URL.Host // Set the Host header to match the target host
	}

	loggedProxy := logRequest(proxy) // Wrap the proxy with the logging middleware

	return loggedProxy, nil
}

func rewriteProxyPath(targetBasePath string, requestPath string, stripPrefix string, prependPrefix string) string {
	trimmedPath := strings.TrimPrefix(requestPath, stripPrefix)
	if trimmedPath == "" {
		trimmedPath = "/"
	}

	rewrittenPath := joinURLPath(prependPrefix, trimmedPath)
	return joinURLPath(targetBasePath, rewrittenPath)
}

func joinURLPath(base string, suffix string) string {
	switch {
	case base == "" && suffix == "":
		return "/"
	case base == "":
		return ensureLeadingSlash(suffix)
	case suffix == "":
		return ensureLeadingSlash(base)
	default:
		return ensureLeadingSlash(path.Join(base, suffix))
	}
}

func ensureLeadingSlash(value string) string {
	if value == "" {
		return "/"
	}

	if strings.HasPrefix(value, "/") {
		return value
	}

	return "/" + value
}

func createProxyMux(doubaoTarget string) (*http.ServeMux, error) {
	doubaoProxy, err := createReverseProxy(doubaoTarget, "/doubao", "")
	if err != nil {
		return nil, err
	}

	openRouterProxy, err := createReverseProxy(openRouterTarget, "/openrouter", "/api")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("/doubao/v1/", doubaoProxy)
	mux.Handle("/openrouter/v1/", openRouterProxy)

	return mux, nil
}

func main() {
	target := os.Getenv("FORWARD_URL")
	if target == "" {
		log.Fatal("FORWARD_URL environment variable is required")
	}

	mux, err := createProxyMux(target)
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "80" // Default port if not specified
	}

	log.Printf("Starting reverse proxy server on port %s", port)
	log.Printf("Doubao namespace: /doubao/v1/* -> %s/v1/*", target)
	log.Printf("OpenRouter namespace: /openrouter/v1/* -> %s/api/v1/*", openRouterTarget)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
