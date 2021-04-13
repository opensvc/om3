package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/client/api"
)

func TestNewGetDaemonStatusHasCustomValues(t *testing.T) {
	c, _ := client.New(client.WithURL("https://localhost:1215"))
	a := api.NewGetDaemonStatus(c).SetSelector("foo").SetNamespace("ns1")
	assert.Equal(t, a.Selector(), "foo")
	assert.Equal(t, a.Namespace(), "ns1")
}

func TestNewGetDaemonStatusHasDefaultNullNamespace(t *testing.T) {
	c, _ := client.New(client.WithURL("https://localhost:1215"))
	a := api.NewGetDaemonStatus(c)
	assert.Equal(t, a.Namespace(), "")
}
