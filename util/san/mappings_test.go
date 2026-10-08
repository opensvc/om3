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

// TestParseMappingsReadsRealInitiatorNames covers the iqn an initiator is
// really named with, which holds colons of its own. Cut at its first colon,
// the initiator lost its end to the targets, and no target was found.
func TestParseMappingsReadsRealInitiatorNames(t *testing.T) {
	const (
		hba1 = "iqn.1993-08.org.debian:01:abcdef"
		hba2 = "iqn.1993-08.org.debian:01:123456"
		tgt1 = "iqn.2005-10.org.freenas.ctl:tgt1"
		tgt2 = "iqn.2005-10.org.freenas.ctl:tgt2"
	)
	paths, err := ParseMappings([]string{hba1 + ":" + tgt1 + "," + tgt2, hba2 + ":" + tgt1})
	require.NoError(t, err)
	require.Len(t, paths, 3)
	assert.Equal(t, hba1, paths[0].Initiator.Name)
	assert.Equal(t, tgt1, paths[0].Target.Name)
	assert.Equal(t, hba1, paths[1].Initiator.Name)
	assert.Equal(t, tgt2, paths[1].Target.Name)
	assert.Equal(t, hba2, paths[2].Initiator.Name)
	assert.Equal(t, tgt1, paths[2].Target.Name)
	for _, p := range paths {
		assert.Equal(t, ISCSI, p.Initiator.Type)
		assert.Equal(t, ISCSI, p.Target.Type)
	}
}

// TestParseMappingsRefusesAnAmbiguousIQNMapping keeps a mapping that can be
// cut in two places from being cut in the one nobody meant.
func TestParseMappingsRefusesAnAmbiguousIQNMapping(t *testing.T) {
	for _, s := range []string{
		"iqn.1993-08.org.debian:01:abcdef:iqn.2005-10.org.freenas.ctl:iqn.x",
		":iqn.2005-10.org.freenas.ctl:tgt1",
		"iqn.1993-08.org.debian:01:abcdef:iqn.2005-10.org.freenas.ctl:tgt1,",
	} {
		_, err := ParseMappings([]string{s})
		assert.Errorf(t, err, "%q must be refused", s)
	}
}

// The mappings a pool hands to an array name the initiator first, as the
// array commands read them back.
func TestMappingListRoundTrips(t *testing.T) {
	in := []string{
		"5001438002a3004a:50060e8007e2b100,50060e8007e2b101",
		"iqn.1993-08.org.debian:01:abcdef:iqn.2005-10.org.freenas.ctl:tgt1",
	}
	paths, err := ParseMappings(in)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"5001438002a3004a:50060e8007e2b100",
		"5001438002a3004a:50060e8007e2b101",
		"iqn.1993-08.org.debian:01:abcdef:iqn.2005-10.org.freenas.ctl:tgt1",
	}, paths.MappingList())
	back, err := ParseMappings(paths.MappingList())
	require.NoError(t, err)
	assert.Equal(t, paths, back)
}
