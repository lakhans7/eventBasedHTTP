package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/myproject/internal/handler"
	"github.com/myproject/internal/logger"
	"github.com/myproject/internal/queue"
	"log"
)

/* 
Main function for starting the HTTP server and setting up necessary components 
*/
func main() {
	/* Initialize the logger */
	/* This function sets up the logging system for the application 
	 * It uses Zerolog for structured logging, providing easy-to-read logs.
	 */
	logger.InitLogger()

	/* Initialize the queue */
	/* The queue is a mechanism where incoming events are temporarily stored before 
	 * being processed by the worker pool. This could be a Redis, RabbitMQ, or in-memory queue.
	 * In this example, we're using a simple in-memory queue.
	 */
	queue.InitQueue()

	/* Create a new Fiber instance */
	/* Fiber is a web framework used to handle HTTP requests. It’s based on fasthttp, which makes it 
	 * one of the fastest Go web frameworks. We’re creating a new Fiber app instance here.
	 */
	app := fiber.New()

	/* Register routes for HTTP messages */
	/* These are the different HTTP methods (GET, POST, PUT, PATCH, DELETE) 
	 * that will handle the corresponding routes. Each route triggers the specific handler function.
	 */

	/* Register routes for GET requests */
	/* This route will handle GET requests sent to "/event". It triggers the 
	 * HandleGetEvent function, which will process the request and return a response.
	 */
	app.Get("/event", handler.HandleGetEvent)

	/* Register routes for POST requests */
	/* This route will handle POST requests sent to "/event". It triggers the 
	 * HandlePostEvent function, where we create an event and push it into the queue.
	 */
	app.Post("/event", handler.HandlePostEvent)

	/* Register routes for PUT requests */
	/* This route will handle PUT requests sent to "/event". It triggers the 
	 * HandlePutEvent function, which will be similar to POST but will update existing events.
	 */
	app.Put("/event", handler.HandlePutEvent)

	/* Register routes for PATCH requests */
	/* This route will handle PATCH requests sent to "/event". It triggers the 
	 * HandlePatchEvent function, typically used for partial updates of existing events.
	 */
	app.Patch("/event", handler.HandlePatchEvent)

	/* Register routes for DELETE requests */
	/* This route will handle DELETE requests sent to "/event". It triggers the 
	 * HandleDeleteEvent function, where we delete events.
	 */
	app.Delete("/event", handler.HandleDeleteEvent)

	/* Start the Fiber server */
	/* The app.Listen(":3000") command starts the server, which listens on port 3000. 
	 * If the server is unable to start, it will log an error and exit.
	 */
	log.Fatal(app.Listen(":3000"))
}
