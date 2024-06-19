package prom

const (
	GAUGE     = "gauge"
	COUNTER   = "counter"
	INFO      = "info"
	HISTOGRAM = "histrogram"
	SUMMARY   = "summary"
	WINDOW    = "windowStats" //librdkafka window stats
	OBJECT    = "object"
	METRICS   = "metrics"
	PREFIX    = "librdkafka_"
	LABELS    = "labels"

	CGRP       = "consumergroups_"
	EOS        = "eos_"
	TOPICS     = "topics_"
	PARTITIONS = "partitions_"
	BROKERS    = "brokers_"
)

var ROOT_LABELS = []string{"client_id", "name", "type"}

var STRING_LABELS = []string{"client_id", "name", "type"}
var FLOAT_LABELS = []string{"ts", "time"}
