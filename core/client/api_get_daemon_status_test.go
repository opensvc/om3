package client

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewGetDaemonStatusHasCustomValues(t *testing.T) {
	c, _ := New(URL("https://localhost:1215"))
	a, _ := NewGetDaemonStatus(c, WithSelector("foo"), WithNamespace("ns1"))
	assert.Equal(t, a.SelectorValue(), SelectorType("foo"))
	assert.Equal(t, string(a.NamespaceValue()), "ns1")
}

func TestNewGetDaemonStatusHasDefaultNullNamespace(t *testing.T) {
	c, _ := New(URL("https://localhost:1215"))
	a, _ := NewGetDaemonStatus(c)
	assert.Equal(t, a.NamespaceValue(), NamespaceType(""))
}
