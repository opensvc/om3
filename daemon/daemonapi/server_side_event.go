package daemonapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"

	"opensvc.com/opensvc/core/event"
)

type (
	eventer interface {
		Event() string
	}

	byter interface {
		Bytes() []byte
	}

	Event event.Event
)

func setStreamHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
}

// writeEvents dequeue c and write sse to w when deueued c is an eventer
func writeEvents(ctx context.Context, w io.Writer, c <-chan any, limit uint64) <-chan error {
	errC := make(chan error)
	// TODO: write comment periodically ': prevent timeout'
	go func() {
		var eventCount uint64 = 0
		for {
			select {
			case <-ctx.Done():
				errC <- nil
				return
			case i := <-c:
				switch o := i.(type) {
				case eventer:
					eventCount++
					ev := Event{
						Kind: o.Event(),
						ID:   eventCount,
					}
					switch d := i.(type) {
					case byter:
						ev.Data = d.Bytes()
					}
					if _, err := ev.write(ctx, w); err != nil {
						errC <- err
						return
					}
					if limit > 0 && eventCount >= limit {
						errC <- nil
						return
					}
				default:
					// drop non eventer
				}
			}
		}
	}()
	return errC
}

// write then event to w
//
// when w is http.ResponseWriter the event is written with
// html Server-sent events format
func (e *Event) write(ctx context.Context, w io.Writer) (int, error) {
	var httpBody bool
	var b []byte
	written := 0

	if _, ok := w.(http.ResponseWriter); ok {
		httpBody = true
	}
	if httpBody {
		b = append(b, []byte("event: "+e.Kind+"\nid: "+strconv.FormatUint(e.ID, 10))...)
		if len(e.Data) > 0 {
			b = append(b, []byte("\ndata: ")...)
			b = append(b, bytes.Replace(e.Data, []byte{'\n'}, []byte("\ndata: "), -1)...)
		}
		b = append(b, []byte("\n\n")...)
	} else {
		b = append(b, e.Data...)
		b = append(b, []byte("\n\n\x00")...)
	}

	select {
	case <-ctx.Done():
		return written, ctx.Err()
	default:
	}
	bLen := len(b)
	for {
		n, err := w.Write(b[written:bLen])
		if err != nil {
			written += n
			return written, err
		}
		if written == bLen {
			break
		}
	}
	i, err := w.Write(b)
	if err != nil {
		return i, err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return written, nil
}
