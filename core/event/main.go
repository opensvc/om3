package event

// Event describes a opensvc daemon event
type Event struct {
	Kind      string      `json:"kind"`
	ID        uint64      `json:"id"`
	Timestamp float64     `json:"ts"`
	Data      interface{} `json:"data"`
}
