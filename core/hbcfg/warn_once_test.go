package hbcfg

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/plog"
)

// A warning about the configuration of a heartbeat is logged once, until it
// changes, the heartbeats being configured again every 30s.
func TestWarnOnce(t *testing.T) {
	var buf bytes.Buffer
	log := plog.NewLogger(zerolog.New(&buf).Level(zerolog.InfoLevel))
	WarnOnce(log, "hb#9", "readjust timeout: %s", "15s => 4m1s")
	WarnOnce(log, "hb#9", "readjust timeout: %s", "15s => 4m1s")
	WarnOnce(log, "hb#8", "readjust timeout: %s", "15s => 4m1s")
	WarnOnce(log, "hb#9", "readjust timeout: %s", "9s => 2m1s")
	assert.Equal(t, 3, strings.Count(buf.String(), "readjust timeout"), buf.String())
}

// The heartbeats remembered are bounded.
func TestWarnOnceIsBounded(t *testing.T) {
	log := plog.NewLogger(zerolog.Nop())
	for i := range 3 * warnedMax {
		WarnOnce(log, "hb#"+strconv.Itoa(i), "readjust timeout")
	}
	warnedMu.Lock()
	defer warnedMu.Unlock()
	assert.LessOrEqual(t, len(warned), warnedMax)
}
