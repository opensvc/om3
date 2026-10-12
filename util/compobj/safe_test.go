package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A safe uri names a safe file by its number or its uuid: the id is a
// component of the path of the collector api it is fetched from, so one
// leading elsewhere in the api is refused.
func TestASafeURINamesASafeFileOnly(t *testing.T) {
	for uri, want := range map[string]string{
		"safe://123":                                  "123",
		"safe://safe.uuid.0123abcdef.conf":            "safe.uuid.0123abcdef.conf",
		"safe://safe.uuid.0123-abcd.tar.gz":           "safe.uuid.0123-abcd.tar.gz",
		"safe://safe.uuid.0123abcdef":                 "safe.uuid.0123abcdef",
		"safe://safe.uuid../../users":                 "",
		"safe://safe.uuid.01/../../users":             "",
		"safe://12/../../users":                       "",
		"safe://safe.uuid.01?x=1":                     "",
		"safe://safe.uuidx":                           "",
		"safe://":                                     "",
		"safe://safe.uuid.0123abcdef.conf/../../../x": "",
	} {
		got, err := collectorSafeURIToUUID(uri)
		if want == "" {
			assert.Error(t, err, uri)
		} else {
			assert.NoError(t, err, uri)
			assert.Equal(t, want, got, uri)
		}
	}
}
