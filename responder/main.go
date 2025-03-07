package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
)

var PORT = flag.Int64("port", 20000, "Provide port to start server on")

// JSON responder api
// If an invalid body is sent
func jsonHandler(res http.ResponseWriter, req *http.Request) {
	bs, err := io.ReadAll(req.Body)
	if err != nil && err != io.EOF {
		log.Println("[jsonHandler] -> error reading body: ", err.Error())
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Check if correct media header is sent
	if req.Header.Get("Content-Type") != "application/json" {
		log.Printf("[jsonHandler] -> incorrect header. Expected: \"application/json\". Received: \"%s\"\n", req.Header.Get("Content-Type"))
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Respond favorably for request with empty body
	if len(bs) == 0 {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write(bs)
		return
	}

	// Check if valid json in case request body is not empty
	var tempVarForSerialization interface{}
	if err := json.Unmarshal(bs, &tempVarForSerialization); err != nil {
		log.Println("[jsonHandler] -> not a valid JSON: ", err.Error())
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(bs)
}

func router(mux *http.ServeMux) {

	// GET /health API
	// always responds with 200 if it can
	mux.HandleFunc("GET /health", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /json", jsonHandler)
}

func main() {
	flag.Parse()
	mux := http.NewServeMux()
	router(mux)
	log.Printf("Starting server at ':%d'\n", *PORT)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *PORT), mux); err != nil {
		log.Fatal("[main] -> Error starting http server: ", err)
	}
}
