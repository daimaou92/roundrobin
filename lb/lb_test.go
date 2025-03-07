package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewInstanceValid(t *testing.T) {
	ins, err := NewInstance("http://localhost:8000/json")
	if err != nil {
		t.Error(err)
	}

	expected := "http://localhost:8000"
	if ins.url != expected {
		t.Errorf("Expected: `%s`, Actual: `%s`\n", expected, ins.url)
	}
}

func TestNewInstanceInvalid(t *testing.T) {
	_, err := NewInstance("http://localhost:8000json")
	if err == nil {
		t.Fatal("method should have returned an error")
	}

	if !strings.HasPrefix(err.Error(), "[NewInstance] -> malformed url: ") {
		t.Error("Wrong error detected or error string has changed")
	}

	_, err = NewInstance("mailto://some@example.org")
	if err == nil {
		t.Fatal("method should have returned an error")
	}

	if !strings.HasPrefix(err.Error(), "[NewInstance] -> Invalid url protocol. Expected: `http`. Actual: ") {
		t.Error("Wrong error detected or error string has changed")
	}
}

func TestInstanceMonitor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
	})

	testServer := httptest.NewUnstartedServer(mux)
	ins, err := NewInstance(fmt.Sprintf("http://%s", testServer.Listener.Addr()))
	if err != nil {
		t.Fatal("error setting up test server: ", err)
	}

	go ins.monitor(t.Context())

	if ins.healthy {
		t.Error("faulty health check. Server hasn't started yet")
	}

	testServer.Start()
	defer testServer.Close()
	time.Sleep(time.Second * 6)
	if !ins.healthy {
		t.Error("faulty health check ticker. Server has started. Should be healthy by now")
	}

	ins.responseTimeCache = []int64{100, 200}
	time.Sleep(time.Second * 2)
	if ins.avgResponseTimeMilli != 150 {
		t.Error("monitor is not calculating server response time average correctly")
	}
}

func TestInstanceIsAvailable(t *testing.T) {
	ins := Instance{
		avgResponseTimeMilli: 10,
		healthy:              false,
	}
	if ins.isAvailable() {
		t.Error("healthy is `false`. isAvailable should return `false`. It returned `true`")
	}

	ins.healthy = true
	if !ins.isAvailable() {
		t.Error("healthy is true and avgResponseTimeMill is less than 100. isAvailable should return `true`. It returned `false`")
	}

	ins.avgResponseTimeMilli = 101
	if ins.isAvailable() {
		t.Error("avgResponseTimeMilli is > 100. isAvailable should return `false`. It returned `true`")
	}
}

func TestInstanceLogResponseTime(t *testing.T) {
	ins, err := NewInstance("http://127.0.0.1:5678")
	if err != nil {
		t.Fatal("error creating new instance: ", err)
	}

	ins.logResponseTime(10, 20)
	if len(ins.responseTimeCache) != 1 {
		t.Error("response time was not appended to ins.responseTimeCache")
	}

	if ins.responseTimeCache[0] != 10 {
		t.Error("response time was calculated incorrectly")
	}

	for i := range 31 {
		ins.logResponseTime(int64(i), int64(2*i+20))
	}

	if len(ins.responseTimeCache) != 20 {
		t.Error("responsetime window of 20 is not beiong maintained correctly")
	}

	if ins.responseTimeCache[19] != 50 {
		t.Error("last value should be 50 not: ", ins.responseTimeCache[19])
	}

	if ins.responseTimeCache[0] != 31 {
		t.Error("first value should be 31 not: ", ins.responseTimeCache[0])
	}
}

func TestInstanceJsonHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /json", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		io.Copy(res, req.Body)
	})

	testServer := httptest.NewServer(mux)
	defer testServer.Close()
	ins, err := NewInstance(fmt.Sprintf("http://%s", testServer.Listener.Addr()))
	if err != nil {
		t.Fatal("error setting up test server: ", err)
	}

	req, err := http.NewRequest(http.MethodPost, "/json", bytes.NewBuffer([]byte{}))
	if err != nil {
		t.Fatal("error creating new request: ", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ins.jsonHandler)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Error("should have received 200. Instead received: ", rr.Code)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Error("handler should be setting `Content-Type: application/json` for 200 response")
	}

	time.Sleep(time.Second * 2)

	if len(ins.responseTimeCache) != 1 {
		t.Error("ins.responseTimeCache should have been filled by now")
	}
}

func TestLBAddInstance(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
	})

	testServer := httptest.NewUnstartedServer(mux)

	lb := LB{
		Ctx: t.Context(),
	}
	err := lb.AddInstance(fmt.Sprintf("http://%s", testServer.Listener.Addr()))
	if err != nil {
		t.Error("There should be no error here. Received: ", err)
	}

	if len(lb.instances) != 1 {
		t.Error("Instance was not created and added correctly")
	}

	testServer.Start()
	defer testServer.Close()

	time.Sleep(time.Second * 6)

	if !lb.instances[0].healthy {
		t.Error("instance monitoring was not started properly")
	}
}

func TestLBGetInstance(t *testing.T) {
	ins1, err := NewInstance("http://localhost:20000")
	if err != nil {
		t.Fatal("NewInstance should not error for this")
	}

	ins2, err := NewInstance("http://localhost:20001")
	if err != nil {
		t.Fatal("NewInstance should not error for this")
	}

	ins3, err := NewInstance("http://localhost:20002")
	if err != nil {
		t.Fatal("NewInstance should not error for this")
	}

	lb := LB{
		Ctx:       t.Context(),
		instances: []*Instance{ins1, ins2, ins3},
	}

	// When all are available RR should be followed
	ins1.healthy = true
	ins2.healthy = true
	ins3.healthy = true

	rIns1 := lb.GetInstance()
	rIns2 := lb.GetInstance()
	rIns3 := lb.GetInstance()
	rIns4 := lb.GetInstance()

	expected := "http://localhost:20001"
	if rIns1.url != expected {
		t.Errorf("Normal Round Robin is not being followed. Expected: `%s`. Actual: `%s`\n", expected, rIns1.url)
	}
	expected = "http://localhost:20002"
	if rIns2.url != expected {
		t.Errorf("Normal Round Robin is not being followed. Expected: `%s`. Actual: `%s`\n", expected, rIns2.url)
	}

	expected = "http://localhost:20000"
	if rIns3.url != expected {
		t.Errorf("Normal Round Robin is not being followed. Expected: `%s`. Actual: `%s`\n", expected, rIns2.url)
	}

	expected = "http://localhost:20001"
	if rIns4.url != expected {
		t.Errorf("Normal Round Robin is not being followed. Expected: `%s`. Actual: `%s`\n", expected, rIns4.url)
	}

	// Round robin should skip unavailable node
	ins1.avgResponseTimeMilli = 101

	rIns1 = lb.GetInstance()
	rIns2 = lb.GetInstance()
	rIns3 = lb.GetInstance()

	expected = "http://localhost:20002"
	if rIns1.url != expected {
		t.Errorf("Round Robin with unavailable node is not being followed. Expected: `%s`. Actual: `%s`\n", expected, rIns1.url)
	}
	expected = "http://localhost:20001"
	if rIns2.url != expected {
		t.Errorf("Round Robin with unavailable node is not being followed. Expected: `%s`. Actual: `%s`\n", expected, rIns2.url)
	}

	expected = "http://localhost:20002"
	if rIns3.url != expected {
		t.Errorf("Round Robin with unavailable node is not being followed. Expected: `%s`. Actual: `%s`\n", expected, rIns2.url)
	}
}
