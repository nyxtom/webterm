package webterm

// LogEvent represents a simple construct for when 'things' occur in the application on certain levels
type LogEvent struct {
	Level   string // level represents a string used for the event level or type
	Message string // message associated with the event occurring
	Err     error  // error potentially associated with this event
	Buf     []byte // buffer of a potential runtime stack
}
