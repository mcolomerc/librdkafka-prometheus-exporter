from confluent_kafka import Producer, Consumer, KafkaException
import string
import random
import sys
import os
import json
from pprint import pformat
import time
import threading    
import logging
import requests

if __name__ == '__main__': 
    broker = os.environ["BOOTSTRAP_SERVERS"]
    topic = os.environ["TOPIC"]
    statsServer = os.environ["STATS_EXPORTER_URL"]

    #Wait to the brokers 
    time.sleep(15)
 
    def delivery_callback(err, msg):
        if err:
            sys.stderr.write('%% Message failed delivery: %s\n' % err)
        else:
            sys.stderr.write('%% Message delivered to %s [%d] @ %d\n' %
                             (msg.topic(), msg.partition(), msg.offset()))
    
    def stats_cb(stats_json_str):
        stats_json = json.loads(stats_json_str)
        print('\nKAFKA Stats: {}\n'.format(pformat(stats_json)))
        r = requests.post(statsServer, json=stats_json)
        print(r.status_code)
    
    def consumer():
        conf = {'bootstrap.servers': broker, 'group.id': 'python-consumer-group', 'session.timeout.ms': 6000,
            'auto.offset.reset': 'earliest', 'enable.auto.offset.store': False}
        
        # Create logger for consumer (logs will be emitted when poll() is called)
        logger = logging.getLogger('consumer')
        logger.setLevel(logging.DEBUG)
        handler = logging.StreamHandler()
        handler.setFormatter(logging.Formatter('%(asctime)-15s %(levelname)-8s %(message)s'))
        logger.addHandler(handler)
        c = Consumer(conf, logger=logger)

        def print_assignment(consumer, partitions):
            print('Assignment:', partitions)

        # Subscribe to topics
        topics = [topic] 
        c.subscribe(topics, on_assign=print_assignment)

        # Read messages from Kafka, print to stdout
        try:
            while True:
                msg = c.poll(timeout=1.0)
                if msg is None:
                  continue
                if msg.error():
                    raise KafkaException(msg.error())
                else:
                    # Proper message
                    sys.stderr.write('%% %s [%d] at offset %d with key %s:\n' %
                                 (msg.topic(), msg.partition(), msg.offset(),
                                  str(msg.key())))
                    print(msg.value())
                    # Store the offset associated with msg to a local cache.
                    # Stored offsets are committed to Kafka by a background thread every 'auto.commit.interval.ms'.
                    # Explicitly storing offsets after processing gives at-least once semantics.
                    c.store_offsets(msg)

        except KeyboardInterrupt:
            sys.stderr.write('%% Aborted by user\n')

        finally:
            # Close down consumer to commit final offsets.
            c.close()

    t1 = threading.Thread(target=consumer) 

    t1.start()
    # Producer configuration 
    conf = {'bootstrap.servers': broker, 'statistics.interval.ms': 5000, 'client.id': 'pyhton-client', 'stats_cb': stats_cb}

    # Create Producer instance
    p = Producer(**conf) 

    # initializing size of string
    N = 10 

    for i in range(1000):
        # generating random strings
        res = ''.join(random.choices(string.ascii_uppercase + str(i) +
                             string.digits, k=N))
 
        try:
            # Produce string
            p.produce(topic, res.rstrip(), callback=delivery_callback)
            time.sleep(0.500)

        except BufferError:
            sys.stderr.write('%% Local producer queue is full (%d messages awaiting delivery): try again\n' %
                             len(p))
 
        p.poll(0)

    # Wait until all messages have been delivered
    sys.stderr.write('%% Waiting for %d deliveries\n' % len(p))
    p.flush()