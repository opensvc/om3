package resdiskxp8

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pairdisplayOutput = `
Group   PairVol L/R  Device_File     Seq# LDEV# P/S Status Fence    % P-LDEV# M
vg_test 9000 L       sdb            541783 36864 P-VOL PAIR NEVER    100 36864 -
vg_test 9000 R       sdb            541782 36864 S-VOL PAIR NEVER    100 36864 -
vg_test 9001 L       sdaa           541783 36865 P-VOL PAIR NEVER    100 36865 -
vg_test 9001 R       sdaa           541782 36865 S-VOL PAIR NEVER    100 36865 -
vg_test 9002 L       sdab           541783 36866 P-VOL PAIR NEVER    100 36866 -
vg_test 9002 R       sdab           541782 36866 S-VOL PAIR NEVER    100 36866 -
`

func TestParsePairdisplay(t *testing.T) {
	t.Run("line count", func(t *testing.T) {
		ps := parsePairdisplay(pairdisplayOutput)
		require.Len(t, ps.Lines, 6, "expected 6 lines (3 pairs × L+R), got %d", len(ps.Lines))
	})

	t.Run("first line fields", func(t *testing.T) {
		ps := parsePairdisplay(pairdisplayOutput)
		l := ps.Lines[0]
		assert.Equal(t, "vg_test", l.Group)
		assert.Equal(t, "9000", l.Volume)
		assert.Equal(t, "sdb", l.DeviceFile)
		assert.Equal(t, "36864", l.LDEV)
		assert.Equal(t, "P-VOL", l.Role)
		assert.Equal(t, "PAIR", l.State)
		assert.Equal(t, "NEVER", l.Fence)
		assert.Equal(t, "100", l.Copied)
		assert.Equal(t, "-", l.M)
	})

	t.Run("L/R flag", func(t *testing.T) {
		ps := parsePairdisplay(pairdisplayOutput)
		// L lines
		assert.True(t, ps.Lines[0].Local, "9000 L should be local")
		assert.True(t, ps.Lines[2].Local, "9001 L should be local")
		assert.True(t, ps.Lines[4].Local, "9002 L should be local")
		// R lines
		assert.False(t, ps.Lines[1].Local, "9000 R should not be local")
		assert.False(t, ps.Lines[3].Local, "9001 R should not be local")
		assert.False(t, ps.Lines[5].Local, "9002 R should not be local")
	})

	t.Run("all devices present", func(t *testing.T) {
		ps := parsePairdisplay(pairdisplayOutput)
		devices := make(map[string]int) // device -> count of L+R lines
		for _, l := range ps.Lines {
			devices[l.Volume]++
		}
		assert.Equal(t, 2, devices["9000"], "9000 should have L and R lines")
		assert.Equal(t, 2, devices["9001"], "9001 should have L and R lines")
		assert.Equal(t, 2, devices["9002"], "9002 should have L and R lines")
	})

	t.Run("statusMap", func(t *testing.T) {
		ps := parsePairdisplay(pairdisplayOutput)
		_, ok := ps.statusMap["PAIR"]
		assert.True(t, ok, "statusMap should contain PAIR")
		assert.Len(t, ps.statusMap, 1, "only one distinct status expected")
	})

	t.Run("roleMap", func(t *testing.T) {
		ps := parsePairdisplay(pairdisplayOutput)
		_, pok := ps.roleMap["P-VOL"]
		_, sok := ps.roleMap["S-VOL"]
		assert.True(t, pok, "roleMap should contain P-VOL")
		assert.True(t, sok, "roleMap should contain S-VOL")
		assert.Len(t, ps.roleMap, 2, "only P-VOL and S-VOL expected")
	})

	t.Run("fenceMap", func(t *testing.T) {
		ps := parsePairdisplay(pairdisplayOutput)
		_, ok := ps.fenceMap["NEVER"]
		assert.True(t, ok, "fenceMap should contain NEVER")
		assert.Len(t, ps.fenceMap, 1, "only one distinct fence value expected")
	})

	t.Run("header line skipped", func(t *testing.T) {
		ps := parsePairdisplay(pairdisplayOutput)
		for _, l := range ps.Lines {
			assert.NotEqual(t, "PairVol", l.Volume, "header line should be skipped")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		ps := parsePairdisplay("")
		assert.Empty(t, ps.Lines)
	})

	t.Run("device files", func(t *testing.T) {
		ps := parsePairdisplay(pairdisplayOutput)
		assert.Equal(t, "sdb", ps.Lines[0].DeviceFile)
		assert.Equal(t, "sdb", ps.Lines[1].DeviceFile)
		assert.Equal(t, "sdaa", ps.Lines[2].DeviceFile)
		assert.Equal(t, "sdab", ps.Lines[4].DeviceFile)
	})
}
