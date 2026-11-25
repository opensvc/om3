package storeplakar

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	// snapshot is one line of plakar ls.
	snapshot struct {
		ID        string
		Timestamp time.Time
		Size      int64
		Root      string
		Tags      map[string]string
	}
)

// parseLs reads the lines of plakar ls -tags, one snapshot each:
//
//	2026-10-15T16:50:17Z   1f988d33    64 KiB        0s /tmp/fix/data tags=om3,lab,app1
//
// The size holds a space, the root may, and the tags are the last field
// when asked for: the line is read from both ends. A tag is a key, or a key
// and a value the driver escaped, so it holds no space nor comma.
func parseLs(out []byte) ([]snapshot, error) {
	l := make([]snapshot, 0)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 6 {
			return nil, fmt.Errorf("plakar ls: unexpected line: %s", line)
		}
		ts, err := time.Parse(time.RFC3339, f[0])
		if err != nil {
			return nil, fmt.Errorf("plakar ls: unexpected line: %s: %w", line, err)
		}
		size, err := sizeconv.FromSize(f[2] + " " + f[3])
		if err != nil {
			return nil, fmt.Errorf("plakar ls: unexpected line: %s: %w", line, err)
		}
		snap := snapshot{
			ID:        f[1],
			Timestamp: ts,
			Size:      size,
			Tags:      make(map[string]string),
		}
		rest := f[5:]
		if last := rest[len(rest)-1]; strings.HasPrefix(last, "tags=") {
			for _, tag := range strings.Split(strings.TrimPrefix(last, "tags="), ",") {
				k, v, _ := strings.Cut(tag, "=")
				if s, err := url.QueryUnescape(v); err == nil {
					v = s
				}
				snap.Tags[k] = v
			}
			rest = rest[:len(rest)-1]
		}
		snap.Root = strings.Join(rest, " ")
		l = append(l, snap)
	}
	return l, nil
}
