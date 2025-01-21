package client

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClientDefaults(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("unexpected error during New: %v", err)
	}
	assert.Condition(t, func() bool { return strings.HasSuffix(c.URL(), ".sock") }, "default url is a ux sock")
}
