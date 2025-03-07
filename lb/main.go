package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

// This is made global for ease. A production grade proxy or LB impl should have it abstracted
// away for better readability
var G_LB *LB

func jsonHandler(res http.ResponseWriter, req *http.Request) {
	instance := G_LB.GetInstance()
	if instance == nil {
		log.Println("[jsonHandler] -> No available instance")
		res.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	instance.jsonHandler(res, req)
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
	mux.HandleFunc("GET /status", nodeStatusHandler)
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
