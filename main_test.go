// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Mock HTTP client and server for testing external requests
var mockHTTPClient *http.Client
var mockHTTPServer *httptest.Server

func TestMain(m *testing.M) {
	// Set Gin to Test Mode
	gin.SetMode(gin.TestMode)

	// Create a mock HTTP server
	mockHTTPServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/append" {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Error reading request body", http.StatusInternalServerError)
				return
			}
			var result map[string]string
			err = json.Unmarshal(body, &result)
			if err != nil {
				http.Error(w, "Error unmarshalling request body", http.StatusBadRequest)
				return
			}
			if result["result"] == "" {
				http.Error(w, "Missing 'result' in request body", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success"}`))
		} else if r.Method == "GET" && r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "UP"}`))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer mockHTTPServer.Close()

	// Create a mock HTTP client that uses the mock server
	mockHTTPClient = mockHTTPServer.Client()
	defaultHTTPClient = mockHTTPClient // Replace the default http client with the mock one
	targetURL = mockHTTPServer.URL + "/append" // Set the target URL to the mock server

	// Set a default POD_NAME for testing
	podName = "test-pod"

	m.Run()
}

// Mockable http.Client
var defaultHTTPClient = &http.Client{}

// This function will be replaced in the test
func mockableDo(req *http.Request) (*http.Response, error) {
	return defaultHTTPClient.Do(req)
}

func TestAppendHandler_Success(t *testing.T) {
	router := gin.Default()
	router.POST("/append", appendHandler)

	jsonInput := `{"input": "hello"}`
	req, _ := http.NewRequest("POST", "/append", bytes.NewBuffer([]byte(jsonInput)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var responseBody map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &responseBody)
	if err != nil {
		t.Fatalf("Error unmarshalling response body: %v", err)
	}

	if responseBody["status"] != "success" {
		t.Errorf("Expected status 'success', got '%s'", responseBody["status"])
	}
}

func TestAppendHandler_InvalidInput(t *testing.T) {
	router := gin.Default()
	router.POST("/append", appendHandler)

	jsonInput := `{"invalid": "hello"}` // Missing "input" field
	req, _ := http.NewRequest("POST", "/append", bytes.NewBuffer([]byte(jsonInput)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHealthHandler_Success(t *testing.T) {
	router := gin.Default()
	router.GET("/health", healthHandler) // Assuming healthHandler is the function name

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var responseBody map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &responseBody)
	if err != nil {
		t.Fatalf("Error unmarshalling response body: %v", err)
	}

	if responseBody["status"] != "UP" {
		t.Errorf("Expected status 'UP', got '%s'", responseBody["status"])
	}
}

func TestAppendHandler_TargetError(t *testing.T) {
	// Temporarily set the mock server to return an error for /append
	originalHandler := mockHTTPServer.Config.Handler
	mockHTTPServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/append" {
			http.Error(w, "Internal Server Error from Target", http.StatusInternalServerError)
		} else {
			originalHandler.ServeHTTP(w, r)
		}
	})
	defer func() {
		mockHTTPServer.Config.Handler = originalHandler // Restore original handler
	}()

	router := gin.Default()
	router.POST("/append", appendHandler)

	jsonInput := `{"input": "hello"}`
	req, _ := http.NewRequest("POST", "/append", bytes.NewBuffer([]byte(jsonInput)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestAppendHandler_TargetNon2xxStatus(t *testing.T) {
	// Temporarily set the mock server to return a non-2xx status for /append
	originalHandler := mockHTTPServer.Config.Handler
	mockHTTPServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/append" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "Bad Request from Target"}`))
		} else {
			originalHandler.ServeHTTP(w, r)
		}
	})
	defer func() {
		mockHTTPServer.Config.Handler = originalHandler // Restore original handler
	}()

	router := gin.Default()
	router.POST("/append", appendHandler)

	jsonInput := `{"input": "hello"}`
	req, _ := http.NewRequest("POST", "/append", bytes.NewBuffer([]byte(jsonInput)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}

	var responseBody map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &responseBody)
	if err != nil {
		t.Fatalf("Error unmarshalling response body: %v", err)
	}

	if responseBody["error"] != "Bad Request from Target" {
		t.Errorf("Expected error message 'Bad Request from Target', got '%s'", responseBody["error"])
	}
}
type statusHandler int

func (h *statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(int(*h))
}

func TestIntegration(t *testing.T) {
	// Set Gin to Test Mode
	gin.SetMode(gin.TestMode)

	// Create a new Gin router
	router := gin.Default()
	router.POST("/append", appendHandler) // Assuming appendHandler is the function name

	// Create a request to the /append endpoint
	jsonInput := `{"input": "hello"}`
	req, _ := http.NewRequest("POST", "/append", bytes.NewBuffer([]byte(jsonInput)))
	req.Header.Set("Content-Type", "application/json")

	// Create a response recorder
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Assert the response status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Assert the response body (you'll need to adjust this based on the actual handler output)
	expectedBodySubstring := "hello I am Java instance default-pod" // Assuming "default-pod" is the default POD_NAME
	if !strings.Contains(w.Body.String(), expectedBodySubstring) {
		t.Errorf("Expected response body to contain \"%s\", got \"%s\"", expectedBodySubstring, w.Body.String())
	}
}
