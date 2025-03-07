package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddInstanceHandlerSuccess(t *testing.T) {
	var err error
	G_LB, err = NewLB(t.Context(), "")
	if err != nil {
		t.Fatal("NewLB should not error here: ", err.Error())
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(addInstanceHandler)

	req, err := http.NewRequest(http.MethodPut, "http://localhost:30000/addinstance", bytes.NewBuffer([]byte(`http://localhost:20000`)))
	if err != nil {
		t.Fatal("NewRequest should not error here: ", err.Error())
	}
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: Expected: `%d`, Actual: `%d`\n", http.StatusOK, rr.Code)
	}

	if len(G_LB.instances) != 1 {
		t.Errorf("Instance did not actually get added")
	}
}

func TestAddInstanceHandlerFailure(t *testing.T) {
	var err error
	G_LB, err = NewLB(t.Context(), "")
	if err != nil {
		t.Fatal("NewLB should not error here: ", err.Error())
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(addInstanceHandler)

	req, err := http.NewRequest(http.MethodPut, "http://localhost:30000/addinstance", bytes.NewBuffer([]byte(`localhost:20000`)))
	if err != nil {
		t.Fatal("NewRequest should not error here: ", err.Error())
	}
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: Expected: `%d`, Actual: `%d`\n", http.StatusBadRequest, rr.Code)
	}

	if len(G_LB.instances) != 0 {
		t.Errorf("Instance somehow got added")
	}
}

func TestNodeStatusHandler(t *testing.T) {
	var err error
	G_LB, err = NewLB(t.Context(), "http://localhost:20000,http://localhost:20001")
	if err != nil {
		t.Fatal("NewLB should not error here: ", err.Error())
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(nodeStatusHandler)

	req, err := http.NewRequest(http.MethodGet, "http://localhost:30000/status", nil)
	if err != nil {
		t.Fatal("NewRequest should not error here: ", err.Error())
	}
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: Expected: `%d`, Actual: `%d`\n", http.StatusOK, rr.Code)
	}

	expected := `{"healthy":[],"available":[],"all":["http://localhost:20000","http://localhost:20001"]}`
	if rr.Body.String() != expected {
		t.Errorf("Status Body failed\nExpected: `%s`\nActual: `%s`\n", rr.Body.String(), expected)
	}
}
