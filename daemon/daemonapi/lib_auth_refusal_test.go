package daemonapi

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/plog"
)

// A client refused authentication at each of its polls is logged once as a
// warning, then once per interval with the count of the refusals between;
// another client, or another reason, is logged on its own.
func TestAuthRefusalsArePaced(t *testing.T) {
	var buf bytes.Buffer
	log := plog.NewLogger(zerolog.New(&buf).Level(zerolog.InfoLevel))
	r := &authRefusals{interval: 10 * time.Minute, m: make(map[string]*authRefusal)}
	noCreds := errors.New("request has no credentials")
	now := time.Now()
	for i := range 41 {
		r.log(log, now.Add(time.Duration(i)*15*time.Second), "[2001:db8::12]:4540"+string(rune('0'+i%10)), "/metrics", noCreds)
	}
	assert.Equal(t, 2, strings.Count(buf.String(), `"level":"warn"`), "the first, then the sum after 10 minutes: %s", buf.String())
	assert.Contains(t, buf.String(), "39 more requests", buf.String())
	assert.NotContains(t, buf.String(), `"level":"error"`)

	buf.Reset()
	r.log(log, now, "[2001:db8::13]:1", "/metrics", noCreds)
	assert.Equal(t, 1, strings.Count(buf.String(), `"level":"warn"`), "another client")
	for i := range 50 {
		r.log(log, now.Add(time.Second), "[2001:db8::13]:1", "/api", errors.New("user u"+strconv.Itoa(i)+" refused"))
	}
	assert.Equal(t, 1, strings.Count(buf.String(), `"level":"warn"`), "a client changing its username at each request is paced too")
}

// A client changing the refused username at each request does not grow the
// clients remembered past the bound.
func TestAuthRefusalsAreBounded(t *testing.T) {
	log := plog.NewLogger(zerolog.Nop())
	r := &authRefusals{interval: 10 * time.Minute, m: make(map[string]*authRefusal)}
	now := time.Now()
	for i := range 3 * authRefusalsMax {
		r.log(log, now.Add(time.Duration(i)*time.Millisecond), "[2001:db8::12]:1", "/api", errors.New("user u"+strconv.Itoa(i)+" refused"))
	}
	assert.LessOrEqual(t, len(r.m), authRefusalsMax)
}
