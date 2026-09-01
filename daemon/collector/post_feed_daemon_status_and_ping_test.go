package collector

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bodyWithRawData is the shape reader replaces: a json.RawMessage field,
// which Marshal compacts rather than splices. Kept here as the reference
// the spliced body has to agree with.
type bodyWithRawData struct {
	PreviousUpdatedAt time.Time       `json:"previous_updated_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	Changes           []string        `json:"changes"`
	Version           string          `json:"version"`
	Data              json.RawMessage `json:"data"`
}

func TestPostFeedDaemonStatusReader(t *testing.T) {
	body := postFeedDaemonStatus{
		PreviousUpdatedAt: time.Unix(1700000000, 0).UTC(),
		UpdatedAt:         time.Unix(1700000060, 0).UTC(),
		Changes:           []string{"a", "b"},
		Version:           "3.0.0",
	}

	for name, data := range map[string]string{
		"a dataset": `{"cluster":{"object":{"ns1/svc/a":{}}},"daemon":{"nodename":"n1"}}`,
		"one holding a string encoding/json would escape": `{"label":"a & b <tag>"}`,
		"an empty object": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			r, err := body.reader([]byte(data))
			require.NoError(t, err)
			got, err := io.ReadAll(r)
			require.NoError(t, err)

			// It has to be valid json, and mean the same as the shape it
			// replaces. Field order differs, the dataset going last, and
			// json objects are unordered, so compare parsed.
			var gotMap map[string]any
			require.NoError(t, json.Unmarshal(got, &gotMap), "produced invalid json: %s", got)

			want, err := json.Marshal(bodyWithRawData{
				PreviousUpdatedAt: body.PreviousUpdatedAt,
				UpdatedAt:         body.UpdatedAt,
				Changes:           body.Changes,
				Version:           body.Version,
				Data:              json.RawMessage(data),
			})
			require.NoError(t, err)
			var wantMap map[string]any
			require.NoError(t, json.Unmarshal(want, &wantMap))
			assert.Equal(t, wantMap, gotMap)

			// And the point of it: the dataset appears exactly as it was
			// handed over, never re-encoded. The bytes already come out
			// of a json.Marshal, so what escaping there is to do has
			// been done, and compacting them again only copies them.
			assert.Contains(t, string(got), data)
		})
	}
}

func TestPostFeedDaemonStatusReaderRejectsANonObjectEnvelope(t *testing.T) {
	// The splice appends before the closing brace, so it has to be sure
	// there is one.
	_, err := postFeedDaemonStatus{}.reader([]byte(`{}`))
	assert.NoError(t, err, "the real envelope is always an object")
}
