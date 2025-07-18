package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request details
		log.Printf("Received request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

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

func createReverseProxy(target string) (http.Handler, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = req.URL.Host // Set the Host header to match the target host
	}

	loggedProxy := logRequest(proxy) // Wrap the proxy with the logging middleware

	return loggedProxy, nil
}

func main() {
	target := os.Getenv("FORWARD_URL")
	if target == "" {
		log.Fatal("FORWARD_URL environment variable is required")
	}

	proxy, err := createReverseProxy(target)
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "80" // Default port if not specified
	}

	http.Handle("/", proxy)

	log.Printf("Starting reverse proxy server, forwarding requests to %s. Listening on port %s", target, port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
