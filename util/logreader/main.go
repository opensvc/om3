package logreader

import (
	"context"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/streamlog"
	"github.com/opensvc/om3/v3/util/logging"
)

// DefaultMaxEntries is the default maximum number of log entries to keep in memory
const DefaultMaxEntries = 10000

// DefaultDrainInterval is the default interval for periodic draining in follow mode
const DefaultDrainInterval = 500 * time.Millisecond

// logEntry represents a log message with its timestamp and node information
type logEntry struct {
	event       streamlog.Event
	node        string
	time        time.Time
	index       int // for sort stability
	streamIndex int // the index of the stream/node for prefix selection
}

// Config holds configuration for the log reader
type Config struct {
	// MaxEntries is the maximum number of log entries to keep in memory
	MaxEntries int

	// DrainInterval is the interval for periodic draining in follow mode
	DrainInterval time.Duration
}

// Option is a function that modifies Config
type Option func(*Config)

// WithMaxEntries sets the maximum number of entries to keep in memory
func WithMaxEntries(size int) Option {
	return func(c *Config) {
		if size > 0 {
			c.MaxEntries = size
		}
	}
}

// WithDrainInterval sets the drain interval for follow mode
func WithDrainInterval(d time.Duration) Option {
	return func(c *Config) {
		if d > 0 {
			c.DrainInterval = d
		}
	}
}

// WithHeapSize is deprecated, use WithMaxEntries instead
func WithHeapSize(size int) Option {
	return WithMaxEntries(size)
}

// NodeStream represents a single node's log stream
type NodeStream struct {
	Node   string
	Reader event.ReadCloser
	Index  int // Position index for prefix selection (0-based)
}

// LogReader manages multi-node log reading with in-memory sorting
type LogReader struct {
	config  Config
	entries []logEntry
	mu      sync.Mutex
	index   int
	stop    chan struct{}
	wg      sync.WaitGroup
}

// New creates a new LogReader with the given options
func New(opts ...Option) *LogReader {
	c := Config{
		MaxEntries:    DefaultMaxEntries,
		DrainInterval: DefaultDrainInterval,
	}
	for _, opt := range opts {
		opt(&c)
	}

	return &LogReader{
		config:  c,
		entries: make([]logEntry, 0, c.MaxEntries),
		stop:    make(chan struct{}),
	}
}

// Start begins reading from all provided node streams
// It first collects the backlog, sorts it, and outputs it
// If follow is enabled, it continues with periodic draining
func (lr *LogReader) Start(streams []NodeStream, output io.Writer, renderer RenderFunc, follow bool) {
	// Channel for sorted entries
	sorted := make(chan logEntry, lr.config.MaxEntries)

	// Start all node readers
	for _, stream := range streams {
		lr.wg.Add(1)
		go lr.readNode(stream, follow)
	}

	// If follow mode, start periodic drainer
	if follow {
		lr.wg.Add(1)
		go lr.followManager(sorted)
	} else {
		// For non-follow mode, wait for all readers to finish and drain once
		lr.wg.Add(1)
		go func() {
			lr.wg.Done()
			lr.wg.Wait()
			// Drain all collected entries
			lr.drain(sorted, 0)
			close(sorted)
		}()
	}

	// Process sorted entries
	for entry := range sorted {
		if err := renderer(entry.event, entry.node, entry.streamIndex, output); err != nil {
			// If we can't write, stop processing
			break
		}
	}
}

// readNode reads from a single node and adds entries to the slice
func (lr *LogReader) readNode(stream NodeStream, follow bool) {
	defer lr.wg.Done()

	for {
		select {
		case <-lr.stop:
			return
		default:
			event, err := stream.Reader.Read()
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return
			}
			if err != nil {
				// For follow mode, continue on error
				if follow {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return
			}

			rec, err := streamlog.NewEvent(event.Data)
			if err != nil {
				continue
			}

			// Extract timestamp from event
			timestamp := lr.extractTimestamp(rec)

			if timestamp.IsZero() {
				continue
			}

			// Create log entry
			entry := logEntry{
				event:       rec,
				node:        stream.Node,
				time:        timestamp,
				index:       lr.nextIndex(),
				streamIndex: stream.Index,
			}

			// Add to slice
			lr.addEntry(entry)
		}
	}
}

// followManager manages the follow mode behavior with periodic draining
func (lr *LogReader) followManager(out chan logEntry) {
	defer lr.wg.Done()

	// Add initial delay to give the subsystem time to collect and sort incoming data
	time.Sleep(lr.config.DrainInterval)

	ticker := time.NewTicker(lr.config.DrainInterval)
	defer ticker.Stop()

	// Drain after initial delay
	lr.drain(out, 500*time.Millisecond)

	for {
		select {
		case <-lr.stop:
			// Drain remaining entries before exiting
			lr.drain(out, 0)
			close(out)
			return
		case <-ticker.C:
			lr.drain(out, 500*time.Millisecond)
		}
	}
}

// Stop stops all readers
func (lr *LogReader) Stop() {
	close(lr.stop)
	lr.wg.Wait()
}

// addEntry adds an entry to the slice, dropping oldest if at capacity
func (lr *LogReader) addEntry(entry logEntry) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	// If at capacity, remove oldest entry (first in sorted slice)
	if len(lr.entries) >= lr.config.MaxEntries {
		lr.entries = lr.entries[1:]
	}

	// Insert in sorted position (oldest first) using binary search
	// We maintain the slice sorted to make drain a simple copy operation
	i := sort.Search(len(lr.entries), func(i int) bool {
		if lr.entries[i].time.Equal(entry.time) {
			return lr.entries[i].index >= entry.index
		}
		return lr.entries[i].time.After(entry.time)
	})
	lr.entries = append(lr.entries, logEntry{})
	copy(lr.entries[i+1:], lr.entries[i:])
	lr.entries[i] = entry
}

// drain sends entries from the slice that are older than the cutoff time
func (lr *LogReader) drain(out chan logEntry, minAge time.Duration) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if len(lr.entries) == 0 {
		return
	}

	// Calculate cutoff time
	cutoff := time.Now().Add(-minAge)

	// Find the split point: first entry that is newer than cutoff
	// Since entries are sorted oldest first, we can use binary search
	splitIndex := sort.Search(len(lr.entries), func(i int) bool {
		return lr.entries[i].time.After(cutoff)
	})

	// Entries to send are everything before splitIndex
	toSend := lr.entries[:splitIndex]

	// Entries to keep are everything from splitIndex onwards
	lr.entries = lr.entries[splitIndex:]

	// Send entries in chronological order (already sorted)
	for _, entry := range toSend {
		out <- entry
	}
}

// nextIndex returns the next index for sort stability
func (lr *LogReader) nextIndex() int {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	lr.index++
	return lr.index
}

// extractTimestamp extracts the timestamp from a streamlog.Event
func (lr *LogReader) extractTimestamp(event streamlog.Event) time.Time {
	if t, ok := event.M["__REALTIME_TIMESTAMP"].(string); ok {
		if ms, err := strconv.ParseInt(t, 10, 64); err == nil {
			return time.Unix(0, ms*1000)
		}
	}
	return time.Time{}
}

// renderEvent renders a streamlog.Event to the provided writer in the specified format
func renderEvent(e streamlog.Event, node string, streamIndex int, numStreams int, format string, output io.Writer) error {
	switch format {
	case "json":
		_, err := output.Write(e.B)
		if err != nil {
			return err
		}
		_, err = output.Write([]byte("\n"))
		return err
	default:
		// Console format
		w := zerolog.NewConsoleWriter()
		w.Out = output
		w.NoColor = color.NoColor
		w.TimeFormat = "2006-01-02T15:04:05.000000Z07:00"
		w.FormatLevel = logging.FormatLevel
		w.FormatFieldName = func(i any) string { return "" }
		w.FormatFieldValue = func(i any) string { return "" }

		// Determine prefix based on stream index
		prefix := ""
		if node != "" {
			prefixes := []string{"⣇", "⣸"}
			runePosition := streamIndex / 2
			paddingWidth := (numStreams)/2 + 1

			var terminator string

			// test if last rune is not filled with streams
			if numStreams%2 != 0 && streamIndex == numStreams-1 {
				prefix = "⡇"
			} else {
				prefix = prefixes[streamIndex%2]
				if numStreams%2 != 0 {
					terminator = "⡀"
				} else {
					terminator = "⣀"
				}
			}

			// Pad prefix to the calculated width
			prefix = strings.Repeat("⣀", runePosition) + prefix

			if n := paddingWidth - runePosition - 2; n > 0 {
				prefix += strings.Repeat("⣀", paddingWidth-runePosition-2)
			}
			prefix += terminator
		}

		w.FormatMessage = func(i any) string {
			nodePrefix := ""
			if node != "" {
				nodePrefix = rawconfig.Colorize.Bold(prefix) + " " + rawconfig.Colorize.Bold(node+": ")
			}
			return nodePrefix + rawconfig.Colorize.Bold(i)
		}

		// Write the event data
		var err error

		switch s := e.M["JSON"].(type) {
		case string:
			_, err = w.Write([]byte(s))
		}

		return err
	}
}

// RenderFunc is a function that renders a log event to an output writer
// It receives the event, node name, stream index, and output writer
// This allows callers to customize how events are rendered
type RenderFunc func(streamlog.Event, string, int, io.Writer) error

// DefaultRenderFunc returns a default renderer that uses streamlog.Event.Render
// numStreams is the total number of streams for calculating padding width
func DefaultRenderFunc(format string, numStreams int) RenderFunc {
	return func(e streamlog.Event, node string, streamIndex int, w io.Writer) error {
		return renderEvent(e, node, streamIndex, numStreams, format, w)
	}
}

// CollectAndSort collects log entries from multiple nodes, sorts them by timestamp,
// and renders them using the provided renderer function.
//
// This function manages the complete lifecycle:
//  1. For non-follow mode: collects all backlog, sorts it, renders it, then exits
//  2. For follow mode: collects initial backlog, sorts and renders it, then continues
//     with periodic draining every DrainInterval
//
// Parameters:
// - streams: slice of NodeStream, each containing a node name and its log reader
// - output: the io.Writer to write sorted log entries to (can be os.Stderr, a text view, etc.)
// - renderer: function to render each event (receives event, node name, and output writer)
// - follow: whether to continue reading new logs after initial backlog
// - opts: optional configuration options (max entries, drain interval)
//
// Entries are automatically dropped when MaxEntries is reached.
func CollectAndSort(streams []NodeStream, output io.Writer, renderer RenderFunc, follow bool, opts ...Option) {
	if len(streams) == 0 {
		return
	}

	// Create log reader with options
	lr := New(opts...)

	// Start reading and processing
	lr.Start(streams, output, renderer, follow)
}

// CollectAndSortWithFormat is a convenience function that uses the default renderer
// with the specified format string.
//
// Parameters:
// - streams: slice of NodeStream, each containing a node name and its log reader
// - output: the io.Writer to write sorted log entries to (can be os.Stderr, a text view, etc.)
// - format: the output format (e.g., "", "json")
// - follow: whether to continue reading new logs after initial backlog
// - opts: optional configuration options (max entries, drain interval)
func CollectAndSortWithFormat(streams []NodeStream, output io.Writer, format string, follow bool, opts ...Option) {
	CollectAndSort(streams, output, DefaultRenderFunc(format, len(streams)), follow, opts...)
}
