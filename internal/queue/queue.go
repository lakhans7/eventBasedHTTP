package queue

import (
	"log"
	"sync"

	"github.com/myproject/internal/event"
	"github.com/myproject/internal/worker"
)

var eventQueue []event.Event // In-memory slice to store events in the queue
var mu sync.Mutex            // Mutex to ensure thread-safety for queue operations

/*
InitQueue initializes the queue
For now, we are using a simple in-memory queue (a slice).
This function can be replaced with a more robust solution like Redis or RabbitMQ.
*/
func InitQueue() {
	eventQueue = make([]event.Event, 0) // Initialize the eventQueue as an empty slice
}

/*
AddToQueue adds an event to the queue
This function locks the queue with a mutex to ensure thread-safe operations when adding events.
*/
func AddToQueue(e event.Event) {
	mu.Lock()         // Lock the queue to avoid concurrent write access
	defer mu.Unlock() // Ensure the lock is released after the operation

	eventQueue = append(eventQueue, e) // Add event to the queue
	log.Printf("Event added to queue: %v", e)

	/* Once an event is added to the queue, the worker pool will be triggered to process the event */
	worker.DispatchWork()
}

/*
GetQueue retrieves the current state of the queue
For testing purposes, this function returns the list of events currently in the queue.
*/
func GetQueue() []event.Event {
	return eventQueue
}
