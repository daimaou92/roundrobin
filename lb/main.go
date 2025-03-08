package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// This is made global for ease. A production grade proxy or LB impl should have it abstracted
// away for better readability
var G_LB *LB

// Metrics
var RESPONSE_DURATION_METRIC = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "avg_response_duration_millis",
	Help: "Average response duration guage data in milliseconds",
}, []string{"instance"})

var RESPONSE_STATUS_METRIC = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "response_status",
	Help: "Response status code",
}, []string{"status"})

func jsonHandler(res http.ResponseWriter, req *http.Request) {
	instance := G_LB.GetInstance()
	if instance == nil {
		log.Println("[jsonHandler] -> No available instance")
		RESPONSE_STATUS_METRIC.WithLabelValues(fmt.Sprintf("%d", http.StatusServiceUnavailable)).Inc()
		res.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	if err := instance.jsonHandler(res, req); err != nil {
		// Retry - in a second and a half need
		// not be here and can be abstracted away if more than 1 retry is needed
		time.Sleep(time.Millisecond * 1500)
		instance := G_LB.GetInstance()
		if instance == nil {
			log.Println("[jsonHandler] -> No available instance")
			RESPONSE_STATUS_METRIC.WithLabelValues(fmt.Sprintf("%d", http.StatusServiceUnavailable)).Inc()
			res.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if err := instance.jsonHandler(res, req); err != nil {
			RESPONSE_STATUS_METRIC.WithLabelValues(fmt.Sprintf("%d", http.StatusServiceUnavailable)).Inc()
			res.WriteHeader(http.StatusServiceUnavailable)
		}
	}
}

func addInstanceHandler(res http.ResponseWriter, req *http.Request) {
	instanceUrl, err := io.ReadAll(req.Body)
	if err != nil {
		log.Println("[addInstanceHandler] -> error reading request body", err)
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := G_LB.AddInstance(string(instanceUrl)); err != nil {
		log.Println("[addInstanceHandler] -> ", err.Error())
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	res.WriteHeader(http.StatusOK)
}

func removeInstanceHandler(res http.ResponseWriter, req *http.Request) {
	instanceUrl, err := io.ReadAll(req.Body)
	if err != nil {
		log.Println("[removeInstanceHandler] -> error reading request body", err)
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	G_LB.RemoveInstance(string(instanceUrl))
	res.WriteHeader(http.StatusOK)
}

func nodeStatusHandler(res http.ResponseWriter, req *http.Request) {
	healthy := []string{}
	available := []string{}
	all := []string{}
	for _, v := range G_LB.instances {
		all = append(all, v.url)

		if v.healthy {
			healthy = append(healthy, v.url)
		}

		if v.isAvailable() {
			available = append(available, v.url)
		}
	}
	resp := struct {
		Healthy   []string `json:"healthy"`
		Available []string `json:"available"`
		All       []string `json:"all"`
	}{
		Healthy:   healthy,
		Available: available,
		All:       all,
	}
	bs, err := json.Marshal(resp)
	if err != nil {
		log.Println("[healthyNodes] -> error marshalling json: ", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(bs)
}

func router(mux *http.ServeMux) {
	mux.HandleFunc("POST /json", jsonHandler)
	mux.HandleFunc("PUT /addinstance", addInstanceHandler)
	mux.HandleFunc("PUT /removeinstance", removeInstanceHandler)
	mux.HandleFunc("GET /status", nodeStatusHandler)
	mux.Handle("/metrics", promhttp.Handler())
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("[main] -> Error loading env vars: ", err)
	}

	mainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// initialize global instance of Load Balancer
	var err error
	G_LB, err = NewLB(mainCtx, os.Getenv("LB_INSTANCELIST"))
	if err != nil {
		log.Fatal("[main] -> ", err.Error())
	}

	mux := http.NewServeMux()
	router(mux)

	log.Println("Starting server at ':30000'")
	if err := http.ListenAndServe(":30000", mux); err != nil {
		log.Fatal("[main] -> err starting server: ", err)
	}
}
