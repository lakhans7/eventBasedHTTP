package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/myproject/internal/event"
	"github.com/myproject/internal/logger"
	"github.com/myproject/internal/queue"
)

/*
HandleGetEvent handles GET requests to "/event"
This function can be used to fetch a list of events or event details if necessary.
*/
func HandleGetEvent(c *fiber.Ctx) error {
	// Your GET request logic (e.g., retrieving events or other data)
	return c.SendString("GET Event Handler") // Respond with a simple string message
}

/*
HandlePostEvent handles POST requests to "/event"
This function generates a new event from the incoming POST request and adds it to the queue.
*/
func HandlePostEvent(c *fiber.Ctx) error {
	/*
		Generate a new event for the POST request
		We generate a unique event ID using UUID, define its type as "CREATE",
		and add some details for the event.
	*/
	e := event.Event{
		ID:     uuid.New().String(),         // Generate a unique ID for the event
		Type:   "CREATE",                    // Event type, can be dynamic based on the request
		Detail: "Details of the POST event", // Event details can come from request body
	}

	/*
		Log the event creation
		Structured logging is done using Zerolog here.
	*/
	logger.GetLogger().Info().Str("event_id", e.ID).Msg("Received new event")

	/*
		Add the event to the queue for processing
		Once added to the queue, a worker will pick up the task and process it.
	*/
	queue.AddToQueue(e)

	/*
		Return a response to the client
		We respond with a JSON message containing the status and event details.
	*/
	return c.JSON(fiber.Map{"status": "Event received", "event": e})
}

/*
HandlePutEvent, HandlePatchEvent, HandleDeleteEvent can be implemented similarly to POST
Each will process the corresponding HTTP request type and perform the necessary event handling.
*/
