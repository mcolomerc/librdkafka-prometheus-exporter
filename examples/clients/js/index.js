const Kafka = require('@confluentinc/kafka-javascript');
const axios = require('axios');

function sleep(millis) {
    return new Promise(resolve => setTimeout(resolve, millis));
}

async function producerStart() {

    //counter to stop this sample after maxMessages are sent
    let counter = 0;
    let maxMessages = 1000;

    const producer = new Kafka.Producer({
        'metadata.broker.list': process.env.BOOTSTRAP_SERVERS,
        'acks': 'all',
        'statistics.interval.ms': '10000',
        'dr_cb': true,
        'client.id': 'js-client'
    });
    producer.setPollInterval(100);
    producer.on('event.stats', function (stats) {
        console.log('producer event.stats: ', stats.message);
        statsToExporter(stats.message);
    })

    //Wait for the ready event before producing
    producer.on('ready', async function (arg) {
        console.log('producer ready.' + JSON.stringify(arg));

        for (var i = 0; i < maxMessages; i++) {
            var value = Buffer.from('value-' + i);
            var key = "key-" + i;
            // if partition is set to -1, librdkafka will use the default partitioner
            var partition = -1;
            var headers = [
                { header: "header value" }
            ]
            await sleep(1000)
            console.log('\n produce :: message :: ' + JSON.stringify(value));
            producer.produce(process.env.TOPIC, partition, value, key, Date.now(), "", headers);
        }

        //need to keep polling for a while to ensure the delivery reports are received
        var pollLoop = setInterval(function () {
            producer.poll();
            if (counter === maxMessages) {
                clearInterval(pollLoop);
                producer.disconnect();
            }
        }, 1000);

    });
    //starting the producer
    producer.connect()

        .on('delivery-report', function (err, report) {
            console.log('delivery-report: ' + JSON.stringify(report));
            counter++;
        })
        .on('event.error', function (err) {
            console.error('Error from producer');
            console.error(err);
        })
        .on('disconnected', function (arg) {
            console.log('producer disconnected. ' + JSON.stringify(arg));
        });


}

async function consumerStart() {

    // Initialization
    const consumer = new Kafka.KafkaConsumer({
        'metadata.broker.list': process.env.BOOTSTRAP_SERVERS,
        'group.id': process.env.GROUP_ID,
        'auto.offset.reset': 'earliest',
        'enable.partition.eof': 'true',
        'statistics.interval.ms': '15000',
        'client.id': 'js-client'
    });

    //logging debug messages, if debug is enabled
    consumer.on('event.log', function (log) {
        console.log(log);
    });

    //logging all errors
    consumer.on('event.error', function (err) {
        console.error('Error from consumer');
        console.error(err);
    });

    //counter to commit offsets every numMessages are received  
    consumer.on('ready', function (arg) {
        console.log('consumer ready.' + JSON.stringify(arg));
        consumer.subscribe([process.env.TOPIC]);
        //start consuming messages
        consumer.consume();
    });


    consumer.on('data', function (m) {
        console.log('consume :: message :: ' + JSON.stringify(m));
        console.log(m.value.toString());
    });

    consumer.on('disconnected', function (arg) {
        console.log('consumer disconnected. ' + JSON.stringify(arg));
    });

    consumer.on('event.stats', function (stats) {
        console.log('consumer event.stats: ', stats.message);
        statsToExporter(stats.message);
    });

    //starting the consumer
    consumer.connect();
}

function statsToExporter(stats) {
    st = JSON.parse(stats)
    axios.post(process.env.STATS_EXPORTER_URL, st)
        .then(response => {
            console.log(response.data);
        })
        .catch(error => {
            console.log(error);
        });
}

async function start() {
    await sleep(15000) // Wait for the broker to be ready. 
    producerStart();
    consumerStart();
}

start();