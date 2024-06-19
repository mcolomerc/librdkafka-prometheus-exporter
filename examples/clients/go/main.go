package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

// from https://github.com/confluentinc/confluent-kafka-go/blob/master/examples

func main() {
	topic := os.Getenv("TOPIC")

	bootstrap := os.Getenv("BOOTSTRAP_SERVERS")

	topics := []string{topic}

	time.Sleep(15 * time.Second)

	// Kafka Producer
	go kafkaProducer(bootstrap, topic)

	// Kafka Consumer
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Consumer
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":      bootstrap,
		"group.id":               "simple-golang-consumer",
		"client.id":              "go-client",
		"auto.offset.reset":      "earliest",
		"statistics.interval.ms": 15000,
	})
	if err != nil {
		log.Printf("Failed to create consumer: %s\n", err)
		os.Exit(1)
	}

	log.Printf("Created Consumer %v\n", c)

	err = c.SubscribeTopics(topics, nil)
	if err != nil {
		fmt.Printf("Failed to subscribe to topic: %v\n", err)
		os.Exit(1)
	}
	run := true
	for run == true {
		select {
		case sig := <-sigchan:
			fmt.Printf("Caught signal %v: terminating\n", sig)
			run = false
		default:
			ev := c.Poll(100)
			if ev == nil {
				continue
			}
			switch e := ev.(type) {
			case *kafka.Message:
				log.Printf("%% Consumed Message on %s:\n%s\n",
					e.TopicPartition, string(e.Value))
				if e.Headers != nil {
					log.Printf("%% Headers: %v\n", e.Headers)
				}
			case kafka.Error:
				fmt.Fprintf(os.Stderr, "%% Error: %v\n", e)
			case *kafka.Stats:
				go pushMetrics(e)
			default:
				log.Printf("Ignored %v\n", e)
			}
		}
	}

	log.Printf("Closing consumer\n")
	c.Close()
}

func kafkaProducer(bootstrapServers string, topic string) {
	// Producer
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":      bootstrapServers,
		"client.id":              "go-client",
		"statistics.interval.ms": 10000,
	})
	if err != nil {
		fmt.Printf("Failed to create producer: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created Producer %v\n", p)

	// Listen to all the events on the default events channel
	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				m := ev
				if m.TopicPartition.Error != nil {
					log.Printf("Delivery failed: %v\n", m.TopicPartition.Error)
				} else {
					log.Printf("Delivered message to topic %s [%d] at offset %v\n",
						*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
				}
			case kafka.Error:
				fmt.Printf("Error: %v\n", ev)
			case *kafka.Stats:
				// Producer performance statistics
				pushMetrics(ev)
			default:
				log.Printf("Ignored event: %s\n", ev)
			}
		}
	}()

	msgcnt := 0
	for msgcnt < 1000 {
		value := fmt.Sprintf("Producer example, message #%d", msgcnt)

		err = p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          []byte(value),
			Headers:        []kafka.Header{{Key: "myTestHeader", Value: []byte("header values are binary")}},
		}, nil)

		if err != nil {
			if err.(kafka.Error).Code() == kafka.ErrQueueFull {
				// Producer queue is full, wait 1s for messages
				// to be delivered then try again.
				time.Sleep(time.Second)
				continue
			}
			log.Printf("Failed to produce message: %v\n", err)
		}
		msgcnt++
		time.Sleep(100 * time.Millisecond)
	}

	// Flush and close the producer and the events channel
	for p.Flush(10000) > 0 {
		log.Print("Still waiting to flush outstanding messages\n")
	}
	p.Close()
}

func pushMetrics(ev *kafka.Stats) error {
	fmt.Println("Pushing stats ")
	// push metric to the HTTP Server
	statsServer := os.Getenv("STATS_EXPORTER_URL")
	log.Println(statsServer)
	var stats map[string]interface{}
	json.Unmarshal([]byte(ev.String()), &stats)

	// Convert the map to a byte array
	buf, err := json.Marshal(stats)
	if err != nil {
		fmt.Println(err)
		return err
	}
	log.Println("Sending stats")
	// Craete the HTTP Client
	// Push the metric to the HTTP Server
	httpClient := &http.Client{}
	// Create a new request
	req, err := http.NewRequest(http.MethodPost, statsServer, bytes.NewReader(buf))
	if err != nil {
		fmt.Println(err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("\n Error sending request: %v\n", err)
		return err
	}
	req.Close = true
	defer resp.Body.Close()
	return nil
}
