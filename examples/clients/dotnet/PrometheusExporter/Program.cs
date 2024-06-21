

using System.Text;
using Confluent.Kafka;
using Confluent.Kafka.Admin;
using Newtonsoft.Json;

string broker = Environment.GetEnvironmentVariable("BOOTSTRAP_SERVERS") ?? throw new InvalidOperationException();
string topic = Environment.GetEnvironmentVariable("TOPIC") ?? throw new InvalidOperationException();
string statsServer = Environment.GetEnvironmentVariable("STATS_EXPORTER_URL") ?? throw new InvalidOperationException();
HttpClient httpClient = new HttpClient();



// Wait for the brokers
Thread.Sleep(15000);

var config = new AdminClientConfig { BootstrapServers = broker };

using var adminClient = new AdminClientBuilder(config).Build();
        
try
{
    await CreateTopicIfNotExists(adminClient, topic);
    Console.WriteLine($"Topic '{topic}' is ready.");
}

catch (CreateTopicsException e)
{
    Console.WriteLine($"An error occured creating topic {e.Results[0].Topic}: {e.Results[0].Error.Reason}");
}

// Start consumer thread
var consumerThread = new Thread(() => ConsumerTask());
consumerThread.Start();

// Producer configuration
var producerConfig = new ProducerConfig
{
    BootstrapServers = broker,
    StatisticsIntervalMs = 5000,
    ClientId = "dotnet-client",
    AllowAutoCreateTopics = true
};

using var producer = new ProducerBuilder<Null, string>(producerConfig)
    .SetStatisticsHandler((_, json) => StatsCallback(json))
    .Build();

Random random = new Random();
int N = 10;

for (int i = 0; i < 1000; i++)
{
    // generating random strings
    string res = new string(Enumerable.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZ" + i + "0123456789", N)
        .Select(s => s[random.Next(s.Length)]).ToArray());

    try
    {
        var dr = await producer.ProduceAsync(topic, new Message<Null, string> { Value = res.Trim() });
        Console.WriteLine($"Delivered '{dr.Value}' to '{dr.TopicPartitionOffset}'");
        Thread.Sleep(500);
    }
    catch (ProduceException<Null, string> e)
    {
        Console.WriteLine($"Delivery failed: {e.Error.Reason}");
    }
}

producer.Flush(TimeSpan.FromSeconds(10));


void StatsCallback(string statsJsonStr)
{
    var statsJson = JsonConvert.DeserializeObject<Dictionary<string, object>>(statsJsonStr);
    Console.WriteLine($"KAFKA Stats: {JsonConvert.SerializeObject(statsJson, Formatting.Indented)}");
    var content = new StringContent(JsonConvert.SerializeObject(statsJson), Encoding.UTF8, "application/json");
    var response = httpClient.PostAsync(statsServer, content).Result;
    Console.WriteLine(response.StatusCode);
}

void ConsumerTask()
{
    var consumerConfig = new ConsumerConfig
    {
        BootstrapServers = broker,
        GroupId = "dotnet-consumer-group",
        AutoOffsetReset = AutoOffsetReset.Earliest,
        EnableAutoOffsetStore = false,
        SessionTimeoutMs = 6000,
        AllowAutoCreateTopics = true
    };

    using var consumer = new ConsumerBuilder<Ignore, string>(consumerConfig)
        .SetLogHandler((_, logMessage) => Console.WriteLine($"Consumer Log: {logMessage.Message}"))
        .Build();

    consumer.Subscribe(topic);

    try
    {
        while (true)
        {
            var consumeResult = consumer.Consume();
            if (consumeResult != null)
            {
                Console.WriteLine($"Consumed message '{consumeResult.Message.Value}' at: '{consumeResult.TopicPartitionOffset}'.");
                consumer.StoreOffset(consumeResult);
            }
        }
    }
    catch (OperationCanceledException)
    {
        consumer.Close();
    }
}

async Task CreateTopicIfNotExists(IAdminClient adminClient, string topicName)
{
    // Check if the topic already exists
    var metadata = adminClient.GetMetadata(TimeSpan.FromSeconds(10));
    bool topicExists = metadata.Topics.Exists(t => t.Topic == topicName);

    if (!topicExists)
    {
        // Create the topic
        var topicSpecification = new TopicSpecification
        {
            Name = topicName,
            NumPartitions = 1, // Set the number of partitions
            ReplicationFactor = 1 // Set the replication factor
        };

        await adminClient.CreateTopicsAsync(new[] { topicSpecification });
        Console.WriteLine($"Topic '{topicName}' created.");
    }
    else
    {
        Console.WriteLine($"Topic '{topicName}' already exists.");
    }
}