package commoncmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The help documents every pg_* keyword of the store, each on one line.
func TestTheCapHelpListsEveryPGKeyword(t *testing.T) {
	long := capLong("svc")
	for _, kw := range pgKeywordsOf("svc") {
		assert.Contains(t, long, "  "+kw.Option+" ", kw.Option)
	}
	assert.Contains(t, long, "pg_pids_max")
	assert.Contains(t, long, "pg_mem_high")
}

func TestACapSummaryHoldsOnOneLine(t *testing.T) {
	for _, kw := range pgKeywordsOf("svc") {
		s := strings.TrimSuffix(strings.TrimSuffix(capSummary(kw.Text), " (unified only)"), " (v1 only)")
		assert.LessOrEqual(t, len(s), capSummaryWidth+3, kw.Option)
	}
	assert.Contains(t, capSummary("The memory. Only the unified cgroup hierarchy has it."), "(unified only)")
}
