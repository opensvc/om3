package daemonapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

func setStreamHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
}

func writeEvents(ctx context.Context, w io.Writer, c <-chan any, limit int64, eventSource string) <-chan error {
	errC := make(chan error)
	// TODO: write comment periodically ': prevent timeout'
	go func() {
		var eventCount int64 = 0
		for {
			select {
			case <-ctx.Done():
				errC <- nil
				return
			case i := <-c:
				eventCount++
				b := []byte("id: " + eventSource + "\nid: " + strconv.FormatInt(eventCount, 10) + "\n")
				if _, err := w.Write(b); err != nil {
					errC <- err
					return
				}
				if err := writeEvent(ctx, w, i); err != nil {
					errC <- err
					return
				}
				if limit > 0 && eventCount >= limit {
					errC <- nil
					return
				}
			}
		}
	}()
	return errC
}

func writeEvent(ctx context.Context, w io.Writer, ev any) error {
	var httpBody bool
	if _, ok := w.(http.ResponseWriter); ok {
		httpBody = true
	}

	b, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal %v", ev)
	}

	var endMsg, msg []byte
	if httpBody {
		endMsg = []byte("\n\n")
		msg = append([]byte("data: "), b...)
	} else {
		endMsg = []byte("\n\n\x00")
		msg = append([]byte(""), b...)
	}

	msg = append(msg, endMsg...)
	select {
	case <-ctx.Done():
		return nil
	default:
	}
	if _, err := w.Write(msg); err != nil {
		return errors.New("write failure")
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}
