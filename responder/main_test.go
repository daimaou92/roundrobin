package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJsonHandlerEmpty(t *testing.T) {
	req, err := http.NewRequest("POST", "/json", bytes.NewBuffer([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonHandler)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected: %d, Actual: %d\n", http.StatusOK, rr.Code)
	}

	respBody := rr.Body.String()
	if respBody != "" {
		t.Errorf("Expected:, Actual:%s\n", respBody)
	}
}

func TestJsonHandlerHeader(t *testing.T) {
	req, err := http.NewRequest("POST", "/json", bytes.NewBuffer([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/html")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonHandler)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected: %d, Actual: %d\n", http.StatusOK, rr.Code)
	}
}

func TestJsonHandlerNonEmpty(t *testing.T) {
	req, err := http.NewRequest("POST", "/json", bytes.NewBuffer([]byte(`{"a": "b"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonHandler)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected: %d, Actual: %d\n", http.StatusOK, rr.Code)
	}

	expected := `{"a": "b"}`
	respBody := rr.Body.String()
	if respBody != expected {
		t.Errorf("Expected:%s, Actual:%s\n", expected, respBody)
	}
}

func TestJsonHandlerMalformedBody(t *testing.T) {
	req, err := http.NewRequest("POST", "/json", bytes.NewBuffer([]byte(`{"a": "b"`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonHandler)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected: %d, Actual: %d\n", http.StatusOK, rr.Code)
	}
}
