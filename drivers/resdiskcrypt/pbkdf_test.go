//go:build linux

package resdiskcrypt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPbkdfArgs(t *testing.T) {
	cases := map[string]struct {
		t        T
		expected []string
	}{
		"managed passphrase, no keyword": {
			t:        T{ManagePassphrase: true},
			expected: []string{"--pbkdf", "pbkdf2", "--iter-time", "100"},
		},
		"user passphrase, no keyword": {
			t:        T{},
			expected: nil,
		},
		"managed passphrase, iter_time": {
			t:        T{ManagePassphrase: true, IterTime: 2000},
			expected: []string{"--pbkdf", "pbkdf2", "--iter-time", "2000"},
		},
		"managed passphrase, pbkdf2": {
			t:        T{ManagePassphrase: true, Pbkdf: "pbkdf2"},
			expected: []string{"--pbkdf", "pbkdf2", "--iter-time", "100"},
		},
		"managed passphrase, argon2id": {
			t:        T{ManagePassphrase: true, Pbkdf: "argon2id"},
			expected: []string{"--pbkdf", "argon2id"},
		},
		"managed passphrase, pbkdf_memory with no pbkdf": {
			t:        T{ManagePassphrase: true, PbkdfMemory: "65536"},
			expected: []string{"--pbkdf-memory", "65536"},
		},
		"managed passphrase, pbkdf_parallel with no pbkdf": {
			t:        T{ManagePassphrase: true, PbkdfParallel: 1},
			expected: []string{"--pbkdf-parallel", "1"},
		},
		"user passphrase, pbkdf2": {
			t:        T{Pbkdf: "pbkdf2"},
			expected: []string{"--pbkdf", "pbkdf2"},
		},
		"user passphrase, iter_time": {
			t:        T{IterTime: 500},
			expected: []string{"--iter-time", "500"},
		},
		"user passphrase, argon2i capped": {
			t:        T{Pbkdf: "argon2i", PbkdfMemory: "64m", PbkdfParallel: 1, IterTime: 1000},
			expected: []string{"--pbkdf", "argon2i", "--pbkdf-memory", "65536", "--pbkdf-parallel", "1", "--iter-time", "1000"},
		},
		"user passphrase, pbkdf_memory as a size": {
			t:        T{PbkdfMemory: "1g"},
			expected: []string{"--pbkdf-memory", "1048576"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			args, err := c.t.pbkdfArgs()
			require.NoError(t, err)
			assert.Equal(t, c.expected, args)
		})
	}
}

func TestPbkdfArgsRefused(t *testing.T) {
	cases := map[string]struct {
		t       T
		message string
	}{
		"pbkdf2 with pbkdf_memory": {
			t:       T{ManagePassphrase: true, Pbkdf: "pbkdf2", PbkdfMemory: "65536"},
			message: "pbkdf_memory apply to the argon2 key derivations, not to pbkdf = pbkdf2",
		},
		"pbkdf2 with pbkdf_parallel": {
			t:       T{Pbkdf: "pbkdf2", PbkdfParallel: 1},
			message: "pbkdf_parallel apply to the argon2 key derivations, not to pbkdf = pbkdf2",
		},
		"pbkdf2 with both": {
			t:       T{Pbkdf: "pbkdf2", PbkdfMemory: "64m", PbkdfParallel: 1},
			message: "pbkdf_memory and pbkdf_parallel apply to the argon2 key derivations, not to pbkdf = pbkdf2",
		},
		"pbkdf_memory not a size": {
			t:       T{Pbkdf: "argon2id", PbkdfMemory: "lots"},
			message: "pbkdf_memory lots is neither a count of KiB nor a size",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := c.t.pbkdfArgs()
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.message)
		})
	}
}

func TestValidatePbkdfMemory(t *testing.T) {
	for _, s := range []string{"65536", "64m", "1g", "512k"} {
		assert.NoErrorf(t, validatePbkdfMemory(s), "%s", s)
	}
	for _, s := range []string{"0", "-1", "lots", "50%"} {
		assert.Errorf(t, validatePbkdfMemory(s), "%s", s)
	}
}
