package fswatch

import "time"

// Op describes a set of file operations.
type Op int

const (
	// Create indicates a new file was detected.
	Create Op = iota + 1
	// Modify indicates a file's content changed.
	Modify
	// Delete indicates a file was removed.
	Delete
)

// String returns a human-readable representation of the operation.
func (o Op) String() string {
	switch o {
	case Create:
		return "CREATE"
	case Modify:
		return "MODIFY"
	case Delete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

// Event represents a file system change event.
type Event struct {
	// Path is the absolute path of the affected file.
	Path string
	// Op is the operation that triggered the event.
	Op Op
	// ModTime is the last modification time of the file at the time the event was detected.
	ModTime time.Time
}
