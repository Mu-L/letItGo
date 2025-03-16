# LetItGo

<p align="center">
  <img src="/assets/logo.png" alt="LetItGo Logo" width="400"/>
</p>

<p align="center">
  <a href="#features"><img src="https://img.shields.io/badge/status-active-success.svg" alt="Status"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-GNU-blue.svg" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/sumit189/letItGo"><img src="https://goreportcard.com/badge/github.com/sumit189/letItGo" alt="Go Report Card"></a>
  <a href="https://github.com/sumit189/letItGo/issues"><img src="https://img.shields.io/github/issues/sumit189/letItGo.svg" alt="Issues"></a>
</p>

**LetItGo** is a robust, distributed webhook scheduling system built in Go that allows for precise timing of API calls. Whether you need one-time or recurring webhook triggers, LetItGo provides a reliable solution with built-in retry mechanisms and encryption.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Environment Variables](#environment-variables)
  - [Installation](#installation)
- [API Usage](#api-usage)
  - [Schedule a Webhook](#schedule-a-webhook)
  - [Schedule a Recurring Webhook](#schedule-a-recurring-webhook)
  - [Verify a Webhook Endpoint](#verify-a-webhook-endpoint)
  - [Response Formats](#response-formats) 
- [Deployment](#deployment)
- [Security](#security)
  - [Webhook Verification Process](#webhook-verification-process)
  - [Payload Encryption](#payload-encryption)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
  - [Coding Standards](#coding-standards)
- [License](#license)
- [Acknowledgements](#acknowledgements)
- [Contact](#contact)

## Features

- **Scheduled Webhooks**: Schedule one-time webhook calls at specific times with millisecond precision
- **Recurring Webhooks**: Set up recurring webhooks using standard cron expressions
- **Natural Language Processing**: Describe schedules in plain English (e.g., "next Monday at 3 PM", "tomorrow at noon")
- **Webhook Verification**: Security verification for webhook endpoints with HMAC-SHA256 signatures
- **Payload Encryption**: All payloads are encrypted at rest using AES-256 encryption
- **Retry Mechanisms**: Configurable automatic retries for failed webhook calls with exponential backoff
- **Distributed Architecture**: Kafka-based message passing between components for horizontal scaling
- **MongoDB Storage**: Persistent storage of schedules and archives with TTL indexes
- **Redis Caching**: High-performance caching for processed tasks to prevent duplicate delivery
- **Logging & Monitoring**: Comprehensive logging and performance metrics

## Architecture
<p align="center">
  <img src="/assets/letitgo.svg" alt="LetItGo Architecture" width="700"/>
</p>

LetItGo follows a microservices architecture with three main components:

1. **API Service**: Handles HTTP requests for scheduling and webhook verification
   - Endpoint validation and security checks
   - Request processing and database operations
   - Response formatting and error handling

2. **Producer Service**: Polls the database for pending schedules and publishes them to Kafka
   - Efficient batch processing of scheduled tasks
   - Pre-processing of payloads 
   - Message serialization and delivery to Kafka

3. **Consumer Service**: Consumes scheduled tasks from Kafka and executes webhooks
   - Concurrent webhook execution
   - Retry handling for failed attempts
   - Result tracking and archiving

### Technologies Used

- **Go 1.23.2**: Fast, efficient, and reliable backend processing
- **MongoDB**: Document storage for schedule and archive data
- **Redis**: In-memory caching and rate limiting
- **Apache Kafka**: Message queue for reliable task distribution
- **AWS MSK**: Managed Streaming for Kafka (for production deployments)
- **Docker & Docker Compose**: Containerization and local development

## Getting Started

### Prerequisites

- Docker and Docker Compose

That's it! All other dependencies (Go, MongoDB, Redis, Kafka) are handled by the Docker setup.

### Environment Variables

Create a `.env` file in the project root with the following variables:

```
# Database Configuration
MONGODB_URI=mongodb://mongodb:27017
REDIS_ADDRESS=redis:6379
REDIS_PASSWORD=
REDIS_DB=0

# Messaging Configuration
KAFKA_BROKER=kafka:9092

# Application Configuration
ENVIRONMENT=development
PAYLOAD_ENCRYPTION_KEY=your-32-character-aes-key
WEBHOOK_SECRET_KEY=your-webhook-secret-key

# NLP Integration (Optional)
LLM_API_URL=your-llm-api-url
LLM_API_KEY=your-llm-api-key
```

> **Note**: The hostnames (mongodb, redis, kafka) match the service names in docker-compose.yml for containerized deployments. For local development, use localhost instead.

### Installation

#### Using Docker (Recommended)

1. Clone the repository:
```bash
git clone https://github.com/sumit189/letItGo.git
cd letItGo
```

2. Create the `.env` file from the example:
```bash
cp .env.example .env
```

3. Customize environment variables in the `.env` file:
```bash
nano .env
```

4. Build and run with Docker Compose:
```bash
docker-compose up -d
```

This will start all services including MongoDB, Redis, Zookeeper, Kafka, and all LetItGo services.

5. Monitor the logs:
```bash
docker-compose logs -f
```

6. To stop all services:
```bash
docker-compose down
```

## API Usage

### Schedule a Webhook

Schedule a one-time webhook to be triggered at a specific time:

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

#### Response:

```json
{
  "success": true,
  "message": "Webhook scheduled successfully",
  "data": {
    "id": "64f7a1b2c3d4e5f6a7b8c9d0",
    "webhook_url": "https://your-verified-endpoint.com/webhook",
    "schedule_time": "2023-10-01T15:00:00Z",
    "status": "pending"
  }
}
```

### Schedule a Recurring Webhook

Set up a webhook that triggers according to a cron expression:

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

#### Response:

```json
{
  "success": true,
  "message": "Recurring webhook scheduled successfully",
  "data": {
    "id": "64f7a1b2c3d4e5f6a7b8c9d1",
    "webhook_url": "https://your-verified-endpoint.com/webhook",
    "cron_expression": "0 15 * * *",
    "next_run_time": "2023-10-01T15:00:00Z",
    "status": "pending"
  }
}
```

### Verify a Webhook Endpoint

Before scheduling, verify that your webhook endpoint can receive calls properly:

```bash
curl -X POST http://localhost:8081/webhook/verify \
  -H "Content-Type: application/json" \
  -d '{
    "webhook_url": "https://your-endpoint.com/webhook",
    "method_type": "POST"
  }'
```

Your webhook endpoint needs to return the correct signature in the `X-Webhook-Signature` header.

#### Response:

```json
{
  "success": true,
  "message": "Webhook endpoint verified successfully",
  "data": {
    "webhook_url": "https://your-endpoint.com/webhook",
    "verified": true,
    "verification_time": "2023-10-01T12:34:56Z"
  }
}
```

### Response Formats

All API endpoints return responses in the following format:

```json
{
  "success": true|false,
  "message": "Human-readable message",
  "data": {}, // Response data object
  "error": {} // Only present when success is false
}
```

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

### Production Considerations

- Set appropriate values for retry limits and timeouts
- Configure MongoDB replica set for high availability
- Use a production-grade Kafka cluster (AWS MSK recommended)
- Set up monitoring and alerting for the services
- Configure proper logging retention policies

## Security

### Webhook Verification Process

1. When a webhook endpoint is registered, LetItGo sends a verification request with a challenge token
2. The endpoint must respond with a signature computed using HMAC-SHA256 with your webhook secret
3. The signature must be returned in the `X-Webhook-Signature` header
4. Only verified endpoints can receive webhook calls

Example webhook endpoint verification handler:

```go
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    payload, _ := io.ReadAll(r.Body)
    signature := computeHMAC(payload, "your-webhook-secret-key")
    
    w.Header().Set("X-Webhook-Signature", signature)
    w.WriteHeader(http.StatusOK)
}

func computeHMAC(message []byte, key string) string {
    h := hmac.New(sha256.New, []byte(key))
    h.Write(message)
    return hex.EncodeToString(h.Sum(nil))
}
```

### Payload Encryption

- All webhook payloads are encrypted at rest using AES-256 encryption
- Encryption keys should be stored securely and rotated regularly
- The system uses separate keys for payload encryption and webhook signature verification

## Troubleshooting

### Common Issues

**Issue**: Webhook calls are not being executed at the expected times.
**Solution**: Check for time zone issues in your cron expressions or scheduled times. All times are processed in UTC.

**Issue**: MongoDB connection failures.
**Solution**: Verify your MongoDB URI and ensure the database server is accessible from your application.

**Issue**: Kafka connection issues.
**Solution**: Check Kafka broker settings and ensure the topic exists with proper permissions.

**Issue**: Webhook verification failures.
**Solution**: Ensure your endpoint is correctly computing and returning the HMAC-SHA256 signature.

### Logs

To view application logs:

```bash
# For Docker deployment
docker-compose logs -f api
docker-compose logs -f producer
docker-compose logs -f consumer

# For manual installation
tail -f logs/api.log
tail -f logs/producer.log
tail -f logs/consumer.log
```

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
- Use proper error handling and logging
- Ensure backward compatibility when making changes

## License

This project is licensed under the GNU GPL3 License - see the [LICENSE](LICENSE) file for details.

## Acknowledgements

- [Gorilla Mux](https://github.com/gorilla/mux) for HTTP routing
- [IBM Sarama](https://github.com/IBM/sarama) for Kafka client
- [MongoDB Go Driver](https://github.com/mongodb/mongo-go-driver)
- [Redis Go Client](https://github.com/redis/go-redis)
- [AWS MSK IAM SASL Signer](https://github.com/aws/aws-msk-iam-sasl-signer-go)
- [Robfig Cron](https://github.com/robfig/cron) for cron expression handling

## Contact

Sumit Paul - [@SumitPaul18_9](https://x.com/SumitPaul18_9)  
Project Link: [https://github.com/sumit189/letItGo](https://github.com/sumit189/letItGo)
