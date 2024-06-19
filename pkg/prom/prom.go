package prom

import (
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusLibrdKafkaExporter struct {
	MetricsValues map[string]float64
	Metrics       map[string]interface{}
	Registry      *prometheus.Registry
	Prefix        string
	MapMutex      sync.RWMutex
}

func NewPrometheusLibrdKafkaExporter() *PrometheusLibrdKafkaExporter {

	defaultRegistry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = defaultRegistry
	prometheus.DefaultGatherer = defaultRegistry

	metricsMap := getMappings()

	exporter := &PrometheusLibrdKafkaExporter{
		Registry:      defaultRegistry,
		Prefix:        PREFIX,
		MetricsValues: make(map[string]float64),
		Metrics:       make(map[string]interface{}),
	}
	// Build Root metrics
	exporter.BuildMetrics(metricsMap, ROOT_LABELS, PREFIX)

	//Build Brokers metrics
	brokersMap := getBrokerMappings()
	brokersLabels := append(ROOT_LABELS, "broker", "nodeid", "nodename", "source", "state")
	exporter.BuildMetrics(brokersMap, brokersLabels, PREFIX+BROKERS)

	// Build Topic Metrics
	topicLabels := append(ROOT_LABELS, "topic")
	topicMetricsMap := getTopicsMappings()
	exporter.BuildMetrics(topicMetricsMap, topicLabels, PREFIX+TOPICS)

	// Build ConsumerGroup Metrics
	consumerGroupLabels := append(ROOT_LABELS, "state", "join_state", "rebalance_reason")
	consumerGroupMetricsMap := getConsumerGroupMappings()
	exporter.BuildMetrics(consumerGroupMetricsMap, consumerGroupLabels, PREFIX+CGRP)

	//Build EOS
	eosLabels := append(ROOT_LABELS, "idemp_state", "txn_state")
	eosMetricsMap := getEOSMappings()
	exporter.BuildMetrics(eosMetricsMap, eosLabels, PREFIX+EOS)

	return exporter
}

func (exp *PrometheusLibrdKafkaExporter) BuildMetrics(metricsMap []map[string]interface{}, labels []string, prefix string) {
	for _, metric := range metricsMap {
		mtype := metric[TYPE]
		switch mtype {
		case GAUGE: // Gauge
			exp.BuildGauge(prefix+metric[VALUE].(string), metric[HELP].(string), labels)
		case COUNTER:
			exp.BuildCounter(prefix+metric[VALUE].(string), metric[HELP].(string), labels)
		case WINDOW:
			exp.BuildWindowStats(prefix+metric[VALUE].(string), labels)
		case OBJECT:
			childMetrics := metric[METRICS].([]map[string]interface{})
			objectLabels := metric[LABELS].([]string)
			exp.BuildMetrics(childMetrics, append(labels, objectLabels...), prefix+metric[VALUE].(string)+"_")
		}
	}
}

func (exp *PrometheusLibrdKafkaExporter) BuildGauge(name, help string, labels []string) {
	pGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	},
		labels,
	)
	prometheus.MustRegister(pGauge)
	exp.Metrics[name] = pGauge
}

func (exp *PrometheusLibrdKafkaExporter) BuildCounter(name, help string, labels []string) {
	pCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	},
		labels,
	)
	prometheus.MustRegister(pCounter)
	exp.Metrics[name] = pCounter
}

func (exp *PrometheusLibrdKafkaExporter) BuildWindowStats(name string, labels []string) {
	for k, v := range getWindowsStats() {
		exp.BuildGauge(name+"_"+k, v, labels)
	}
}

func getStringLabels(labels []string, obj map[string]interface{}, fields []string) []string {
	strLabels := labels
	for _, f := range fields {
		strLabels = append(strLabels, obj[f].(string))
	}
	return strLabels
}

func getBrokerLabels(labels []string, brokerObj map[string]interface{}) []string {
	brokerLabels := append(labels, brokerObj["name"].(string))
	brokerLabels = append(brokerLabels, strconv.FormatFloat(brokerObj["nodeid"].(float64), 'f', -1, 64))
	brokerLabels = getStringLabels(brokerLabels, brokerObj, []string{"nodename", "source", "state"})
	return brokerLabels
}

func (p *PrometheusLibrdKafkaExporter) UpdateStats(stats map[string]interface{}) error {
	var labels []string

	for _, label := range ROOT_LABELS {
		if stats[label] != nil {
			var labelValue string // Store Label & Vale
			if _, ok := stats[label].(float64); ok {
				labelValue = strconv.FormatFloat(stats[label].(float64), 'f', -1, 64) // Store Label & Vale
			}
			if _, ok := stats[label].(string); ok {
				labelValue = stats[label].(string) // Store Label & Vale
			}
			labels = append(labels, labelValue)
		}
	}

	// Update ROOT metrics
	for key, value := range stats {
		if _, ok := value.(float64); ok {
			p.UpdateMetric(PREFIX+key, value.(float64), labels)
		}
	}
	// Update Broker Metrics
	if brokers, ok := stats["brokers"]; ok {
		for _, broker := range brokers.(map[string]interface{}) {
			brokerObj := broker.(map[string]interface{})
			brokerLabels := getBrokerLabels(labels, brokerObj)
			for key, value := range brokerObj {
				if _, ok := value.(float64); ok {
					p.UpdateMetric(PREFIX+BROKERS+key, value.(float64), brokerLabels)
				} else {
					if _, ok := value.(map[string]interface{}); ok {
						for k := range getWindowsStats() {
							v := value.(map[string]interface{})[k]
							if _, ok := value.(float64); ok {
								p.UpdateMetric(PREFIX+BROKERS+key+"_"+k, v.(float64), brokerLabels)
							}
						}
					}
				}
			}
		}
	}
	// Update Topic Metrics
	if topics, ok := stats["topics"]; ok {
		for _, topic := range topics.(map[string]interface{}) {
			topicObj := topic.(map[string]interface{})
			topicLabels := getStringLabels(labels, topicObj, []string{"topic"})
			for key, value := range topicObj {
				if _, ok := value.(float64); ok {
					p.UpdateMetric(PREFIX+TOPICS+key, value.(float64), topicLabels)
				} else {
					if _, ok := value.(map[string]interface{}); ok {
						for k := range getWindowsStats() {
							v := value.(map[string]interface{})[k]
							if _, ok := value.(float64); ok {
								p.UpdateMetric(PREFIX+TOPICS+key+"_"+k, v.(float64), topicLabels)
							}
						}
					}
				}
				partitions := topicObj["partitions"].(map[string]interface{})
				for _, partition := range partitions {
					partitionObj := partition.(map[string]interface{})
					partitionLabels := append(topicLabels, strconv.FormatFloat(partitionObj["partition"].(float64), 'f', -1, 64))
					partitionLabels = append(partitionLabels, strconv.FormatFloat(partitionObj["broker"].(float64), 'f', -1, 64))
					partitionLabels = append(partitionLabels, strconv.FormatFloat(partitionObj["leader"].(float64), 'f', -1, 64))
					for key, value := range partitionObj {
						if _, ok := value.(float64); ok {
							p.UpdateMetric(PREFIX+TOPICS+PARTITIONS+key, value.(float64), partitionLabels)
						}
					}
				}
			}
		}
	}
	// Update ConsumerGroup Metrics
	if consumerGroup, ok := stats["cgrp"]; ok {
		consumerGroupObj := consumerGroup.(map[string]interface{})
		consumerGroupLabels := getStringLabels(labels, consumerGroupObj, []string{"state", "join_state", "rebalance_reason"})
		for key, value := range consumerGroupObj {
			if _, ok := value.(float64); ok {
				p.UpdateMetric(PREFIX+CGRP+key, value.(float64), consumerGroupLabels)
			}
		}
	}
	if eos, ok := stats["eos"]; ok {
		eosObj := eos.(map[string]interface{})
		eosObjLabels := getStringLabels(labels, eosObj, []string{"idemp_state", "txn_state"})
		for key, value := range eosObj {
			if _, ok := value.(float64); ok {
				p.UpdateMetric(PREFIX+EOS+key, value.(float64), eosObjLabels)
			}
		}
	}
	return nil
}

func (p *PrometheusLibrdKafkaExporter) UpdateMetric(key string, value interface{}, labels []string) {
	if metric, ok := p.Metrics[key]; ok {
		switch metric.(type) {
		case *prometheus.GaugeVec:
			gauge := metric.(*prometheus.GaugeVec)
			gauge.WithLabelValues(labels...).Set(value.(float64))
		case *prometheus.CounterVec:
			valueKey := key
			for _, lab := range labels {
				valueKey = valueKey + lab
			}
			p.MapMutex.Lock()
			counter := metric.(*prometheus.CounterVec)
			var increment float64
			if m, ok := p.MetricsValues[valueKey]; ok {
				increment = value.(float64) - m
			} else {
				increment = value.(float64)
			}
			if increment > 0 {
				counter.WithLabelValues(labels...).Add(increment)
			}
			p.MetricsValues[valueKey] = value.(float64)
			p.MapMutex.Unlock()
		}
	}
}
