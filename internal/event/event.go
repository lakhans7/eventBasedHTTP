package event

/*
Event struct defines the structure of an event
Each event has an ID, type, and some details about the event.
*/
type Event struct {
	ID     string `json:"id"`     // Unique identifier for the event
	Type   string `json:"type"`   // Type of event (e.g., "CREATE", "UPDATE")
	Detail string `json:"detail"` // Additional details about the event
}
