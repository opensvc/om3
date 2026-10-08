package args

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSecrets(t *testing.T) {
	argv := []string{"s1", "set", "--kw", "env.pass", "--value", "secret", "--value=other"}
	masked := MaskSecrets(argv)
	assert.Equal(t, []string{"s1", "set", "--kw", "env.pass", "--value", "xxx", "--value=xxx"}, masked)
	assert.Equal(t, "secret", argv[5], "the argv given is left untouched")
	assert.Equal(t, []string{"--value"}, MaskSecrets([]string{"--value"}), "a trailing --value has nothing to mask")
}

func TestMaskSecretsEnv(t *testing.T) {
	argv := []string{"/usr/bin/om", "s1", "run", "--rid", "task#1", "--env", "TOKEN=secret", "--env=PASS=other", "--env", "NOEQUAL"}
	assert.Equal(t,
		[]string{"/usr/bin/om", "s1", "run", "--rid", "task#1", "--env", "TOKEN=xxx", "--env=PASS=xxx", "--env", "xxx"},
		MaskSecrets(argv),
		"an --env value keeps the name of the variable it sets")
	assert.Equal(t,
		[]string{"om", "--value", "xxx", "--rid", "x"},
		MaskSecrets([]string{"om", "--value", "--env", "--rid", "x"}),
		"the argument following --value is its value, as the flag parser reads it, whatever it looks like")
}
