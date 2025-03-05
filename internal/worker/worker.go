package worker

import (
	"fmt"
	"sync"

	"github.com/myproject/internal/event"
	"github.com/myproject/internal/queue"
)

var wg sync.WaitGroup // WaitGroup used to wait for worker tasks to complete

/*
DispatchWork assigns a task from the queue to a worker
This function is called when new events are added to the queue, and it triggers the worker to process it.
*/
func DispatchWork() {
	/*
		We are using a single worker in this example, but in a production environment,
		multiple workers can be created to process events concurrently.
	*/
	wg.Add(1)         // Increment WaitGroup counter to wait for the worker
	go processEvent() // Start a new goroutine to process the event
}

/*
processEvent processes events from the queue
In this example, it takes the first event from the queue, processes it, and performs the necessary action.
*/
func processEvent() {
	defer wg.Done() // Decrement the WaitGroup counter when the worker is done processing

	/* Lock the queue to prevent concurrent access while reading events */
	mu.Lock()
	defer mu.Unlock()

	// Check if there are any events in the queue
	if len(queue.GetQueue()) > 0 {
		e := queue.GetQueue()[0] // Get the first event from the queue
		// Log event processing
		fmt.Printf("Processing event: %v\n", e)

		// Call handleEvent to perform different actions based on the event type
		handleEvent(e)
	}
}

/*
handleEvent processes an event based on its type
Here, you can add logic for different event types, for example, handling "CREATE", "UPDATE" events.
*/
func handleEvent(e event.Event) {
	/*
		Based on the event type (CREATE, UPDATE, etc.),
		the function executes the appropriate logic for the event.
	*/
	switch e.Type {
	case "CREATE":
		// Process event of type "CREATE"
		fmt.Println("Handling CREATE event")
	case "UPDATE":
		// Process event of type "UPDATE"
		fmt.Println("Handling UPDATE event")
	default:
		// Default case for unknown event types
		fmt.Println("Unknown event type")
	}
}
