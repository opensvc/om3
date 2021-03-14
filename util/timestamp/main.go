package timestamp

import (
	"fmt"
	"time"
)

// T is a unix timestamp with nanosecond precision.
type T int64

// New return a new timestamp for the present date.
func New() T {
	return T(time.Now().UnixNano())
}

func (t T) String() string {
	return fmt.Sprintf("%f", float64(t)/float64(time.Second))
}
