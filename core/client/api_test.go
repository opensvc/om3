package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGetDaemonStatusHasCustomValues(t *testing.T) {
	c, _ := New(WithURL("https://localhost:1215"))
	a := c.NewGetDaemonStatus().SetSelector("foo").SetNamespace("ns1")
	assert.Equal(t, a.Selector(), "foo")
	assert.Equal(t, a.Namespace(), "ns1")
}

func TestNewGetDaemonStatusHasDefaultNullNamespace(t *testing.T) {
	c, _ := New(WithURL("https://localhost:1215"))
	a := c.NewGetDaemonStatus()
	assert.Equal(t, a.Namespace(), "")
}
