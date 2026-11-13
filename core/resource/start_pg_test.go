package resource

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/plog"
)

// pgFailingDriver is a resource whose process groups cannot be capped.
type pgFailingDriver struct {
	log *plog.Logger
}

func (t pgFailingDriver) ApplyPG(context.Context) error {
	return errors.New("pg /opensvc.slice/opensvc-svc.s.slice: pg_cpus=0-512: invalid argument")
}

func (t pgFailingDriver) Log() *plog.Logger { return t.log }

// A capping that cannot be applied is said, and the start goes on, as with the
// v2 agent: the object was down until the keyword was fixed otherwise.
func TestAFailedCappingIsWarnedAboutBeforeAStart(t *testing.T) {
	var buf bytes.Buffer
	r := pgFailingDriver{log: plog.NewLogger(zerolog.New(&buf))}
	applyPGBeforeStart(context.Background(), r)
	assert.Contains(t, buf.String(), `"level":"warn"`)
	assert.Contains(t, buf.String(), "pg /opensvc.slice/opensvc-svc.s.slice: pg_cpus=0-512")
}
