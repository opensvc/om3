package hp3par

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSSHCommandIsTheTestedShape pins the ssh command line this has been run
// with against real hardware: the command is an argument, not a session
// written on the standard input, and there is no setclienv preamble.
//
// The v2 agent opens a session instead and sets csvtable and nohdtot in it.
// Recent firmware answers the shape below, so the callers ask each command for
// its own -csvtable -nohdtot.
func TestSSHCommandIsTheTestedShape(t *testing.T) {
	config := Config{
		Method:   MethodSSH,
		Manager:  "3par1",
		Username: "svcuser",
		KeyFile:  "/root/.ssh/id_3par",
	}
	args, err := config.Command("showsys -csvtable -nohdtot")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"ssh", "-i", "/root/.ssh/id_3par", "svcuser@3par1", "showsys -csvtable -nohdtot",
	}, args)
}

// TestSSHCommandLeavesOutWhatIsNotConfigured pins that an absent key or
// username is left out rather than passed empty, which is how an array
// reached with the ssh configuration of the calling user works.
func TestSSHCommandLeavesOutWhatIsNotConfigured(t *testing.T) {
	args, err := Config{Method: MethodSSH, Manager: "3par1"}.Command("showsys")
	require.NoError(t, err)
	assert.Equal(t, []string{"ssh", "3par1", "showsys"}, args)

	args, err = Config{Method: MethodSSH, Manager: "3par1", Username: "svcuser"}.Command("showsys")
	require.NoError(t, err)
	assert.Equal(t, []string{"ssh", "svcuser@3par1", "showsys"}, args)

	args, err = Config{Method: MethodSSH, Manager: "3par1", KeyFile: "/k"}.Command("showsys")
	require.NoError(t, err)
	assert.Equal(t, []string{"ssh", "-i", "/k", "3par1", "showsys"}, args)
}

// TestCLICommandIsTheTestedShape pins the cli command line: the array is named
// by its manager, the password file is an argument, and the command words are
// arguments of their own.
//
// The v2 agent names the array by its own name and leaves the password file in
// the TPDPWFILE environment variable instead.
func TestCLICommandIsTheTestedShape(t *testing.T) {
	config := Config{
		Method:  MethodCLI,
		Manager: "3par1",
		CLI:     "/opt/3par/bin/cli",
		PWFile:  "/etc/opensvc/3par1.pwf",
	}
	args, err := config.Command("showvv -showcols Name,VV_WWN -csvtable -nohdtot")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/opt/3par/bin/cli", "-sys", "3par1", "-pwf", "/etc/opensvc/3par1.pwf",
		"showvv", "-showcols", "Name,VV_WWN", "-csvtable", "-nohdtot",
	}, args)
}

// TestCLICommandDefaultsToTheCLIOnThePath covers an array whose configuration
// does not say where the binary is.
func TestCLICommandDefaultsToTheCLIOnThePath(t *testing.T) {
	args, err := Config{Method: MethodCLI, Manager: "3par1"}.Command("showsys")
	require.NoError(t, err)
	assert.Equal(t, []string{"cli", "-sys", "3par1", "showsys"}, args)
}

// TestAnUnreachableConfigurationIsRefused pins that a configuration saying too
// little is refused where it is read, not by a command line that would not
// work.
func TestAnUnreachableConfigurationIsRefused(t *testing.T) {
	_, err := Config{Manager: "3par1"}.Command("showsys")
	assert.ErrorIs(t, err, ErrBuildCommand, "no method")

	_, err = Config{Method: "rest", Manager: "3par1"}.Command("showsys")
	assert.ErrorIs(t, err, ErrBuildCommand, "a method this does not know")

	_, err = Config{Method: MethodSSH}.Command("showsys")
	assert.ErrorIs(t, err, ErrBuildCommand, "no manager")

	_, err = Config{Method: MethodCLI}.Command("showsys")
	assert.ErrorIs(t, err, ErrBuildCommand, "no manager")
}

// TestParseCSVReadsWhatTheCLIPrints covers the shape the cli prints when a
// command is asked for -csvtable -nohdtot.
func TestParseCSVReadsWhatTheCLIPrints(t *testing.T) {
	cols := []string{"Name", "VV_WWN", "Prov"}
	rows := ParseCSV("vv1,50002AC000010A1B,tpvv\nvv2,50002AC000020A1B,full", cols)
	require.Len(t, rows, 2)
	assert.Equal(t, "vv1", rows[0]["Name"])
	assert.Equal(t, "full", rows[1]["Prov"])
}

// TestParseCSVDropsWhatTheColumnsDoNotDescribe pins that a line holding one
// value is not read as a row.
func TestParseCSVDropsWhatTheColumnsDoNotDescribe(t *testing.T) {
	rows := ParseCSV("\nvv1,50002AC000010A1B,tpvv\n\ntotal\n", []string{"Name", "VV_WWN", "Prov"})
	require.Len(t, rows, 1)
	assert.Equal(t, "vv1", rows[0]["Name"])
}

// TestParseCSVReadsAShortLineAsFarAsItGoes keeps a row the array printed with
// fewer values than there are columns.
func TestParseCSVReadsAShortLineAsFarAsItGoes(t *testing.T) {
	rows := ParseCSV("vv1,50002AC000010A1B", []string{"Name", "VV_WWN", "Prov"})
	require.Len(t, rows, 1)
	_, ok := rows[0]["Prov"]
	assert.False(t, ok, "a column the line does not reach is absent, not empty")
}

// TestCleanNormalisesWhatTheArrayPrints covers the carriage returns an ssh
// session brings back.
func TestCleanNormalisesWhatTheArrayPrints(t *testing.T) {
	assert.Equal(t, "a\nb", Clean("a\r\nb\r\n\r\n"))
	assert.Equal(t, "a\nb", Clean("a\rb\n\n"))
	assert.Equal(t, "", Clean("\n\n"))
}
