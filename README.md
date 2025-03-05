# Go Event Processing Utility

This Go-based utility provides a simple HTTP server that handles various types of events. It leverages the **Fiber** web framework for HTTP handling, **Zerolog** for structured logging, and an event queue system that processes events using a **worker pool**.

## Features

- **HTTP Routes**: The utility supports various HTTP methods like GET, POST, PUT, PATCH, and DELETE.
- **Event Queue**: When an event is received via HTTP, it is placed into a queue for further processing.
- **Worker Pool**: The worker pool picks up events from the queue and processes them asynchronously.
- **Structured Logging**: Logs are created using **Zerolog**, which outputs structured JSON logs.
- **Extensibility**: Easily extendable to support additional event types, routes, and background workers.

## Architecture Overview

1. **HTTP Server (Fiber)**: Handles incoming HTTP requests.
2. **Event Queue**: Stores events that need to be processed. This can be an in-memory queue, Redis, or any other messaging system.
3. **Worker Pool**: A pool of workers that pull events from the queue and process them asynchronously.
4. **Logging**: All actions, errors, and events are logged using **Zerolog** for structured, JSON-based logging.

## Project Structure

. ├── cmd/ │ └── server/ │ └── main.go # Entry point of the application ├── internal/ │ ├── handler/ │ │ └── event_handlers.go # Handles incoming HTTP requests (GET, POST, PUT, PATCH, DELETE) │ ├── worker/ │ │ └── worker_pool.go # Worker pool that processes events from the queue │ ├── queue/ │ │ └── queue.go # Event queue for storing events before processing │ ├── logger/ │ │ └── logger.go # Logger setup using Zerolog │ └── event/ │ └── event.go # Defines event structure (ID, type, detail) ├── go.mod # Go module file, managing dependencies ├── go.sum # Auto-generated checksum file for dependencies └── README.md # Project documentation


## Prerequisites

- **Go 1.18+**: Make sure you have Go installed. You can download it from [the official site](https://go.dev/dl/).
- **Dependencies**: This project uses several Go packages. These will be installed automatically by Go when you run the project.

## Installation

1. Clone the repository:
   git clone https://github.com/yourusername/go-event-processing.git
   cd go-event-processing

Install dependencies:
go mod tidy
Running the Application
Run the HTTP server:


go run cmd/server/main.go
This will start the server on http://localhost:3000.

Test the API Endpoints:

You can test the following routes using curl or Postman:

GET /event: Fetch details about the event (can be extended later).
POST /event: Create a new event and add it to the queue.
PUT /event: Update an existing event.
PATCH /event: Partially update an event.
DELETE /event: Delete an event.
Example POST request:

curl -X POST http://localhost:3000/event \
     -d '{"type": "CREATE", "detail": "This is a new event"}' \
     -H "Content-Type: application/json"

Response:
{
  "status": "Event received",
  "event": {
    "id": "unique-event-id",
    "type": "CREATE",
    "detail": "This is a new event"
  }
}


Logging
This project uses Zerolog for structured logging. Logs will be printed to the console in JSON format, which makes it easier to integrate with log management tools like ELK (Elasticsearch, Logstash, and Kibana) or Grafana Loki.

Example Log Output:
{
  "level": "info",
  "timestamp": "2025-03-01T12:00:00Z",
  "message": "Received new event",
  "event_id": "unique-event-id"
}

Queue System
The event queue stores incoming events for processing. In this initial version, it uses an in-memory queue. You can extend this to use Redis, RabbitMQ, or any other distributed queue for production environments.

Worker Pool
The worker pool processes events asynchronously. Each worker pulls an event from the queue, processes it, and logs the result. The worker pool can be scaled by increasing the number of workers based on the load.

Extending the Application
1. Add New Event Types
To add new event types, simply update the event.Event struct to accommodate the new type. For example, you can add fields for new event attributes and adjust the event processing logic in the worker pool.

2. Add More Routes
To add more HTTP routes, simply create new handlers in the handler/event_handlers.go file and add them to the Fiber router in cmd/server/main.go.

3. Replace In-Memory Queue
For production, you may want to use a distributed queue like Redis or RabbitMQ. You can implement a new queue system by modifying internal/queue/queue.go.

4. Scale Worker Pool
You can scale the worker pool by adding more worker goroutines or adjusting the size of the worker pool. This can be done in internal/worker/worker_pool.go.

Testing
You can run tests for your application using the Go testing framework:

go test ./...
Unit Tests
Unit tests can be found in the respective directories (e.g., internal/handler, internal/worker, etc.). To test specific modules, you can run:

go test internal/handler
License
This project is licensed under the MIT License - see the LICENSE file for details.

Acknowledgments
Fiber: A fast and lightweight web framework for Go. Fiber Documentation
Zerolog: A zero-allocation JSON logger. Zerolog Documentation
