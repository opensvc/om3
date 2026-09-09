package san

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseMappingsReadsTheCollectorGrammar pins the grammar the collector
// writes: one initiator, the targets to reach it through, and the option
// repeated once per initiator.
func TestParseMappingsReadsTheCollectorGrammar(t *testing.T) {
	paths, err := ParseMappings([]string{"iqn.hba1:iqn.tgt1,iqn.tgt2", "iqn.hba2:iqn.tgt3"})
	require.NoError(t, err)
	require.Len(t, paths, 3, "two targets of the first initiator, one of the second")

	assert.Equal(t, "iqn.hba1", paths[0].Initiator.Name)
	assert.Equal(t, "iqn.tgt1", paths[0].Target.Name)
	assert.Equal(t, "iqn.hba1", paths[1].Initiator.Name)
	assert.Equal(t, "iqn.tgt2", paths[1].Target.Name)
	assert.Equal(t, "iqn.hba2", paths[2].Initiator.Name)
	assert.Equal(t, "iqn.tgt3", paths[2].Target.Name)

	for _, p := range paths {
		assert.Equal(t, ISCSI, p.Initiator.Type)
		assert.Equal(t, ISCSI, p.Target.Type)
	}
}

// TestParseMappingsReadsFibreChannel covers the wwn form, which has no iqn
// prefix to say what it is.
func TestParseMappingsReadsFibreChannel(t *testing.T) {
	paths, err := ParseMappings([]string{"20000000c9a1b2c3:50060e8010539b01,50060e8010539b02"})
	require.NoError(t, err)
	require.Len(t, paths, 2)
	assert.Equal(t, FC, paths[0].Initiator.Type)
	assert.Equal(t, FC, paths[0].Target.Type)
	assert.Equal(t, "50060e8010539b02", paths[1].Target.Name)
}

// TestParseMappingsRefusesWhatItCannotRead keeps a malformed mapping from
// being read as a mapping to nothing.
func TestParseMappingsRefusesWhatItCannotRead(t *testing.T) {
	for _, s := range []string{"iqn.hba1", "iqn.hba1:", ":iqn.tgt1", "iqn.hba1:iqn.tgt1,"} {
		_, err := ParseMappings([]string{s})
		assert.Errorf(t, err, "%q must be refused", s)
	}

	// Nothing to read is not an error, it is no mapping.
	paths, err := ParseMappings(nil)
	require.NoError(t, err)
	assert.Empty(t, paths)
}

// TestParseMappingLosesTargets is what the singular parser does with a
// collector mapping, and the reason ParseMappings exists.
func TestParseMappingLosesTargets(t *testing.T) {
	paths, _ := ParseMapping("iqn.hba1:iqn.tgt1,iqn.tgt2")
	assert.NotEqual(t, 2, len(paths),
		"the comma separates whole pairs here, so a collector mapping does not survive it")
}
