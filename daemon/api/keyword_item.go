package api

import (
	"encoding/json"
)

// WithEvaluatedText returns the items with the text of each evaluated value
// set, for the items a daemon of an earlier version answered without it: the
// value as it came, which is what those daemons showed.
func WithEvaluatedText(t KeywordItems) KeywordItems {
	for i, item := range t {
		if item.EvaluatedText != nil || item.Evaluated == nil {
			continue
		}
		var text string
		switch v := (*item.Evaluated).(type) {
		case string:
			text = v
		default:
			b, err := json.Marshal(v)
			if err != nil {
				continue
			}
			text = string(b)
		}
		t[i].EvaluatedText = &text
	}
	return t
}
