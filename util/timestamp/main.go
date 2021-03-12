package timestamp

import (
	"fmt"
	"time"
)

type T int64

func New() T {
	return T(time.Now().UnixNano())
}

func (t T) String() string {
	return fmt.Sprintf("%f", float64(t)/float64(time.Second))
}
