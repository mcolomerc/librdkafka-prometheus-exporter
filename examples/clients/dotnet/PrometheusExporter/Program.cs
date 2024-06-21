

using System.Text;
using Confluent.Kafka;
using Newtonsoft.Json;

string broker = Environment.GetEnvironmentVariable("BOOTSTRAP_SERVERS") ?? throw new InvalidOperationException();
string topic = Environment.GetEnvironmentVariable("TOPIC") ?? throw new InvalidOperationException();
string statsServer = Environment.GetEnvironmentVariable("STATS_EXPORTER_URL") ?? throw new InvalidOperationException();
HttpClient httpClient = new HttpClient();



// Wait for the brokers
Thread.Sleep(15000);

// Start consumer thread
var consumerThread = new Thread(() => ConsumerTask());
consumerThread.Start();

// Producer configuration
var producerConfig = new ProducerConfig
{
    BootstrapServers = broker,
    StatisticsIntervalMs = 5000,
    ClientId = "dotnet-client"
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
        SessionTimeoutMs = 6000
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