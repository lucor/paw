package migration

import (
	"fmt"
	"time"
)

// timestamp converts a set date to Unix seconds, discarding subsecond precision.
func timestamp(value time.Time) (*uint64, error) {
	if value.IsZero() {
		return nil, nil
	}
	seconds := value.Unix()
	if seconds < 0 {
		// CXF timestamps are unsigned; older dates cannot be represented.
		return nil, fmt.Errorf("date precedes the Unix epoch")
	}
	result := uint64(seconds)
	return &result, nil
}
