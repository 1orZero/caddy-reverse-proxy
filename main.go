package main

import (
	"bytes"
	"io"
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
		log.Printf("=== REQUEST ===")
		log.Printf("Method: %s", r.Method)
		log.Printf("URL: %s", r.URL.String())
		log.Printf("Remote: %s", r.RemoteAddr)
		
		// Log request headers
		log.Printf("Request Headers:")
		for key, values := range r.Header {
			for _, value := range values {
				log.Printf("  %s: %s", key, value)
			}
		}

		// Read and log request body
		var requestBody []byte
		if r.Body != nil {
			requestBody, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			if len(requestBody) > 0 {
				log.Printf("Request Body: %s", string(requestBody))
			}
		}

		// Record the start time
		start := time.Now()

		// Create a custom response writer to capture the response
		responseWriter := &responseWriter{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
		}

		// Call the original handler
		handler.ServeHTTP(responseWriter, r)

		// Calculate the request duration
		duration := time.Since(start)

		// Log the response details
		log.Printf("=== RESPONSE ===")
		log.Printf("Status: %d", responseWriter.statusCode)
		log.Printf("Duration: %s", duration)
		
		// Log response headers
		log.Printf("Response Headers:")
		for key, values := range w.Header() {
			for _, value := range values {
				log.Printf("  %s: %s", key, value)
			}
		}
		
		// Log response body
		if responseWriter.body.Len() > 0 {
			log.Printf("Response Body: %s", responseWriter.body.String())
		}
		
		log.Printf("=== END ===\n")
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
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
