//go:build linux

package rescontainerkvm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVsockCID(t *testing.T) {
	for s, want := range map[string]string{
		`<vsock model='virtio'><cid auto='yes' address='3'/></vsock>`: "3",
		`<vsock model="virtio"><cid auto="no" address="42"/></vsock>`: "42",
		`<cid address='7' auto='no'/>`:                                "7",
	} {
		got, ok := vsockCID([]byte(s))
		assert.True(t, ok, s)
		assert.Equal(t, want, got, s)
	}
	for _, s := range []string{
		`<domain></domain>`,
		`<vsock model='virtio'><cid auto='yes'/></vsock>`,
	} {
		_, ok := vsockCID([]byte(s))
		assert.False(t, ok, s)
	}
}
