package main

import (
	"encoding/json"
	"log"
	"mcolomerc/librdkafka-prometheus-exporter/pkg/prom"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var promExp *prom.PrometheusLibrdKafkaExporter

const PORT = "PORT"

func main() {
	promExp = prom.NewPrometheusLibrdKafkaExporter()

	http.HandleFunc("/", requestHandler)
	http.Handle("/metrics", promhttp.Handler())

	port := os.Getenv(PORT)

	if port == "" {
		port = "8080"
	}
	log.Println("Listening on port: ", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		handlePost(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handlePost(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	log.Println(">> Handling stats from requester:: ", r.Header.Get("User-Agent"))
	stats := make(map[string]interface{})
	err := json.NewDecoder(r.Body).Decode(&stats) // Pass a pointer to Stats
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("ERROR"))
	}

	errUpd := promExp.UpdateStats(stats)
	if errUpd != nil {
		log.Println(errUpd)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("ERROR"))
	}
	log.Println("Request Completed")
	w.WriteHeader(http.StatusOK) // 200
	w.Write([]byte("OK"))

}
