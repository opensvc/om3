package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/api"
)

// A cancel of the context of GetReader interrupts a stream the daemon does
// not answer: the request used to be made with a background context, and a
// command could not stop before the daemon answered.
func TestGetReaderCancelsAStreamNotAnswering(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	c, err := api.NewClient(server.URL)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := NewGetEvents(c).GetReader(ctx)
		done <- err
	}()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("GetReader did not return once its context was done")
	}
}

// SetReplayDone asks the ReplayDone event, and the reader tells whether the
// daemon answering sends it, as its OM-Replay-Done header says.
func TestGetReaderReplayDone(t *testing.T) {
	for _, advertised := range []bool{true, false} {
		var query string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query = r.URL.RawQuery
			if advertised {
				w.Header().Set(api.HeaderReplayDone, "true")
			}
			w.WriteHeader(http.StatusOK)
		}))
		c, err := api.NewClient(server.URL)
		require.NoError(t, err)
		reader, err := NewGetEvents(c).SetReplay(true).SetReplayDone(true).GetReader(context.Background())
		require.NoError(t, err)
		require.Contains(t, query, "replay_done=true")
		a, ok := reader.(interface{ ReplayDoneAdvertised() bool })
		require.True(t, ok)
		require.Equal(t, advertised, a.ReplayDoneAdvertised())
		_ = reader.Close()
		server.Close()
	}
}
