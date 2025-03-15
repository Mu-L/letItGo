# LetItGo

<p align="center">
  <img src="/assets/logo.png" alt="LetItGo Logo" width="200"/>
</p>

LetItGo is a robust, distributed webhook scheduling system built in Go that allows for precise timing of API calls. Whether you need one-time or recurring webhook triggers, LetItGo provides a reliable solution with built-in retry mechanisms and encryption.

## Features

- **Scheduled Webhooks**: Schedule one-time webhook calls at specific times
- **Recurring Webhooks**: Set up recurring webhooks using cron expressions
- **Natural Language Processing**: Describe schedules in plain English (e.g., "next Monday at 3 PM")
- **Webhook Verification**: Security verification for webhook endpoints
- **Payload Encryption**: All payloads are encrypted at rest
- **Retry Mechanisms**: Automatic retries for failed webhook calls
- **Distributed Architecture**: Kafka-based message passing between components
- **MongoDB Storage**: Persistent storage of schedules and archives
- **Redis Caching**: High-performance caching for processed tasks

## Architecture

LetItGo follows a microservices architecture with three main components:

1. **API Service**: Handles HTTP requests for scheduling and webhook verification
2. **Producer Service**: Polls the database for pending schedules and publishes them to Kafka
3. **Consumer Service**: Consumes scheduled tasks from Kafka and executes webhooks

### Technologies Used

- Go 1.23.2
- MongoDB
- Redis
- Apache Kafka
- AWS MSK (Managed Streaming for Kafka)

## Getting Started

### Prerequisites

- Go 1.23.2 or higher
- MongoDB
- Redis
- Kafka cluster (or AWS MSK)

### Environment Variables

Create a `.env` file in the project root with the following variables:

```
MONGODB_URI=mongodb://localhost:27017
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
KAFKA_BROKER=localhost:9092
ENVIRONMENT=development
PAYLOAD_ENCRYPTION_KEY=your-32-character-aes-key
WEBHOOK_SECRET_KEY=your-webhook-secret-key
LLM_API_URL=your-llm-api-url
LLM_API_KEY=your-llm-api-key
```

### Installation

1. Clone the repository:
```
git clone https://github.com/sumit189/letItGo.git
cd letItGo
```

2. Build the services:
```
./make-build.sh
```

3. Start the services:

For local development:
```
./build/api &
./build/producer &
./build/consumer &
```

For production:
```
./deploy.sh
```

## API Usage

### Schedule a Webhook

```bash
curl -X POST http://localhost:8081/schedule \
  -H "Content-Type: application/json" \
  -d '{
    "webhook_url": "https://your-verified-endpoint.com/webhook",
    "method_type": "POST",
    "payload": {"key": "value"},
    "schedule_time": "2023-10-01T15:00:00Z"
  }'
```

Alternative with natural language time:

```bash
curl -X POST http://localhost:8081/schedule \
  -H "Content-Type: application/json" \
  -d '{
    "webhook_url": "https://your-verified-endpoint.com/webhook",
    "method_type": "POST",
    "payload": {"key": "value"},
    "time_as_text": "next Monday at 3 PM"
  }'
```

### Schedule a Recurring Webhook

```bash
curl -X POST http://localhost:8081/schedule \
  -H "Content-Type: application/json" \
  -d '{
    "webhook_url": "https://your-verified-endpoint.com/webhook",
    "method_type": "POST",
    "payload": {"key": "value"},
    "cron_expression": "0 15 * * *"
  }'
```

### Verify a Webhook Endpoint

```bash
curl -X POST http://localhost:8081/webhook/verify \
  -H "Content-Type: application/json" \
  -d '{
    "webhook_url": "https://your-endpoint.com/webhook",
    "method_type": "POST"
  }'
```

Your webhook endpoint needs to return the correct signature in the `X-Webhook-Signature` header.

## Deployment

For production deployment on Linux systems:

1. Update the deployment script if needed:
```bash
nano deploy.sh
```

2. Run the deployment script:
```bash
sudo ./deploy.sh
```

This will:
- Copy binaries to /usr/local/bin/letItGo/
- Create systemd service files
- Enable and start the services

## Security

- All webhook payloads are encrypted at rest using AES encryption
- Webhook endpoints must be verified before they can receive calls
- The system uses HMAC-SHA256 signatures for webhook verification

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Coding Standards

- Follow Go best practices and code style
- Add comments to explain complex logic
- Write tests for new features
- Update documentation as needed

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgements

- [Gorilla Mux](https://github.com/gorilla/mux) for HTTP routing
- [IBM Sarama](https://github.com/IBM/sarama) for Kafka client
- [MongoDB Go Driver](https://github.com/mongodb/mongo-go-driver)
- [Redis Go Client](https://github.com/redis/go-redis)
- [AWS MSK IAM SASL Signer](https://github.com/aws/aws-msk-iam-sasl-signer-go)

## Contact

Sumit Paul - [@SumitPaul18_9](https://x.com/SumitPaul18_9)
