package sseevent

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/opensvc/om3/v3/core/event"
)

type (
	ReadCloser struct {
		// eventC is the chan where parse() writes found *Event and where
		// Read() fetch *Event
		eventC chan *event.Event

		wrapped io.ReadCloser

		// errC is the go routine error chan
		errC chan error

		// parseStarted become true during first Read (internal go routine is parseStarted)
		parseStarted bool

		// closed is true when Close() is called
		closed bool

		// max is the maxTokenSize for internal scanner
		max int

		// buf is initial scanner buffer for split
		buf []byte

		ctx    context.Context
		cancel context.CancelFunc

		// timeout specifies the maximum duration to wait before Read returns
		// nil if no events are available.
		timeout time.Duration
	}

	Writer struct {
		wrapped io.Writer
	}
)

const (
	// MaxScanTokenSize defines max len of sse event
	MaxScanTokenSize  = 4096 * 1024
	initialBufferSize = 4096
)

var (
	ErrClosed = errors.New("already closed")
)

// NewReadCloser returns *Reader from wrapped reader r.
//
// It starts routine during first Read(), the parseStarted routine returns when
// wrapped r is closed, or when an invalid event is parsed
func NewReadCloser(r io.ReadCloser) *ReadCloser {
	t := &ReadCloser{
		eventC:       make(chan *event.Event),
		errC:         make(chan error),
		wrapped:      r,
		parseStarted: false,
		closed:       false,
		max:          MaxScanTokenSize,
		buf:          make([]byte, initialBufferSize),
	}
	t.ctx, t.cancel = context.WithCancel(context.Background())

	return t
}

// NewTimeoutReadCloser returns NewReadCloser(r io.ReadCloser) where timeout is set.
func NewTimeoutReadCloser(r io.ReadCloser, timeout time.Duration) *ReadCloser {
	t := NewReadCloser(r)
	t.timeout = timeout
	return t
}

// SetContext set reader context, it will replace default reader context,
// it can't be called after initial Read
func (r *ReadCloser) SetContext(ctx context.Context) {
	if r.parseStarted {
		panic("SetContext called after Read")
	}
	r.cancel()
	r.ctx, r.cancel = context.WithCancel(ctx)
}

// Buffer defines buffer value for internal go routine io.Scanner
func (r *ReadCloser) Buffer(buf []byte, max int) {
	if r.parseStarted {
		panic("Buffer called after Read")
	}
	r.buf = buf
	r.max = max
}

// Read returns *Event read from EventReader r
func (r *ReadCloser) Read() (*event.Event, error) {
	if r.closed {
		return nil, ErrClosed
	}
	if !r.parseStarted {
		go r.parse()
		r.parseStarted = true
	}
	ctxTimeout := r.ctx

	if r.timeout > 0 {
		ctx, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		ctxTimeout = ctx
	}
	select {
	case <-r.ctx.Done():
		return nil, r.ctx.Err()
	case err := <-r.errC:
		if err == nil {
			// bug
			return nil, fmt.Errorf("event reader: read unexpected nil from err channel")
		}
		return nil, err
	case e := <-r.eventC:
		if e == nil {
			// bug
			return nil, fmt.Errorf("event reader: read unexpected nil from event channel")
		}
		return e, nil
	case <-ctxTimeout.Done():
		select {
		case <-r.ctx.Done():
			return nil, r.ctx.Err()
		default:
			return nil, nil
		}
	}
}

// Close ask wrapped io.readCloser for Close
func (r *ReadCloser) Close() error {
	if r.closed {
		return ErrClosed
	}
	r.cancel()
	r.closed = true
	return r.wrapped.Close()
}

// parse runs scanner on wrapped reader, parse read lines to construct
// server side event. Write parsed events to eventC.
// return on error, found error is sent to r.errC
func (r *ReadCloser) parse() {
	var (
		scanner = bufio.NewScanner(r.wrapped)

		// fieldSep define the event line field separator
		fieldSep = []byte{':'}

		// leftTrimValue defines the cut set part of field value to remove
		leftTrimValue = " "

		dispatchReady bool

		ev = &event.Event{}
	)
	defer close(r.eventC)
	defer close(r.errC)
	defer r.cancel()
	scanner.Buffer(r.buf, r.max)

	for scanner.Scan() {
		line := scanner.Bytes()

		if len(line) > 0 {
			// not empty line, read fields
			if fieldName, fieldValue, ok := bytes.Cut(line, fieldSep); ok {
				fieldValue = bytes.TrimLeft(fieldValue, leftTrimValue)
				switch string(fieldName) {
				case "":
					// line starts with ':' => ignore, it is a comment
				case "event":
					dispatchReady = true
					ev.Kind = string(fieldValue)
				case "id":
					if id, err := strconv.ParseUint(string(fieldValue), 10, 64); err != nil {
						r.errC <- err
						return
					} else {
						dispatchReady = true
						ev.ID = id
					}
				case "data":
					dispatchReady = true
					// concat multiple consecutive data lines
					ev.Data = append(ev.Data, fieldValue...)
					ev.Data = append(ev.Data, '\n')
				case "retry":
					// drop reconnection field
				case "time":
					if err := ev.At.UnmarshalText(fieldValue); err != nil {
						ev.At = time.Now()
					}
				}
			}
		} else if dispatchReady {
			if ev.At.IsZero() {
				ev.At = time.Now()
			}
			ev.At = time.Now()
			select {
			case <-r.ctx.Done():
				return
			case r.eventC <- ev:
			}

			// reset for next event
			dispatchReady = false
			ev = &event.Event{}
		}
	}
	err := scanner.Err()
	if err == nil {
		err = io.EOF
	}
	select {
	case <-r.ctx.Done():
	case r.errC <- err:
	}
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{wrapped: w}
}

func (w *Writer) Write(ev *event.Event) (int, error) {
	var b []byte
	b = append(b, []byte("event: "+ev.Kind+"\nid: "+strconv.FormatUint(ev.ID, 10))...)
	if len(ev.Data) > 0 {
		b = append(b, []byte("\ndata: ")...)
		b = append(b, bytes.Replace(ev.Data, []byte{'\n'}, []byte("\ndata: "), -1)...)
	}
	b = append(b, []byte("\n\n")...)
	if _, err := w.wrapped.Write(b); err != nil {
		return 0, err
	}
	return 1, nil
}
