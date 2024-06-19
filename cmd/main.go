package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mcolomerc/librdkafka-prometheus-exporter/pkg/prom"
	"os"
)

func main() {
	promExp := prom.NewPrometheusLibrdKafkaExporter()

	jsonFile, err := os.Open("stats.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened users.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)
	// Parse the JSON data
	stats := make(map[string]interface{})

	err = json.Unmarshal(byteValue, &stats) // Pass a pointer to Stats
	if err != nil {
		log.Println(err)
	}
	// Build Prometheus stats
	err = promExp.UpdateStats(stats)
	if err != nil {
		fmt.Println("Error UpdateStats: ", err)
	}

	metrics, err := promExp.Registry.Gather() // Gather the metrics from the registry
	if err != nil {
		fmt.Println("Error gathering metrics: ", err)
	}

	// Print the prometheus metrics
	for _, metric := range metrics {
		fmt.Println("Metric Name: ", metric.GetName())
		fmt.Println("Metric: ", metric.GetMetric())
	}
}
