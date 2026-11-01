package rescontainerkvm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const serialDefinition = `<domain type='kvm'>
  <devices>
    <serial type='pty'>
      <target type='isa-serial' port='0'>
        <model name='isa-serial'/>
      </target>
    </serial>
    <console type='pty'>
      <target type='serial' port='0'/>
    </console>
  </devices>
</domain>
`

func TestDefinitionWithSerialLog(t *testing.T) {
	after := definitionWithSerialLog([]byte(serialDefinition), "/var/log/libvirt/qemu/vm1.serial.log")
	assert.Contains(t, string(after), "<serial type='pty'>\n      <log file='/var/log/libvirt/qemu/vm1.serial.log' append='on'/>\n      <target")
	assert.Equal(t, "/var/log/libvirt/qemu/vm1.serial.log", definedSerialLog(after))
	assert.Equal(t, string(after), string(definitionWithSerialLog(after, "/elsewhere")), "the definition says where already")

	assert.Empty(t, definedSerialLog([]byte(serialDefinition)))
	none := "<domain><devices/></domain>"
	assert.Equal(t, none, string(definitionWithSerialLog([]byte(none), "/f")), "no port to log")
	file := "<domain><serial type='file'><source path='/x'/></serial></domain>"
	assert.Equal(t, file, string(definitionWithSerialLog([]byte(file), "/f")), "writes a file already")
	assert.Contains(t, string(definitionWithSerialLog([]byte(serialDefinition), "/a'b")), "file='/a&#39;b'")
	assert.Equal(t, "/a'b", definedSerialLog(definitionWithSerialLog([]byte(serialDefinition), "/a'b")))
}

func TestCleanLine(t *testing.T) {
	for in, want := range map[string]string{
		"[  \x1b[0;32mOK\x1b[0m  ] Started ssh":    "[  OK  ] Started ssh",
		"\x1b[H\x1b[2J\x1b]0;title\x07login: ":     "login: ",
		"\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\": "link",
		"\x1b[?25l\x1b(Bdone\r":                    "done",
		"tab\there\x07\x00\x7f":                    "tab\there",
		"bad\xff\xfeutf8":                          "badutf8",
		"été \xc2\x9b31m":                          "été 31m",
		"trailing \x1b":                            "trailing ",
	} {
		assert.Equal(t, want, string(cleanLine([]byte(in))), "%q", in)
	}
}

func TestLastLines(t *testing.T) {
	b := []byte("a\nb\nc\n")
	assert.Equal(t, "", string(lastLines(b, 0)))
	assert.Equal(t, "c\n", string(lastLines(b, 1)))
	assert.Equal(t, "b\nc\n", string(lastLines(b, 2)))
	assert.Equal(t, "a\nb\nc\n", string(lastLines(b, 3)))
	assert.Equal(t, "a\nb\nc\n", string(lastLines(b, 9)))
	assert.Equal(t, "c", string(lastLines([]byte("a\nb\nc"), 1)), "a line with no end")
	assert.Empty(t, lastLines(nil, 3))
}

func next(t *testing.T, ch <-chan []byte) string {
	t.Helper()
	select {
	case b, ok := <-ch:
		require.True(t, ok, "closed")
		return string(b)
	case <-time.After(5 * time.Second):
		require.Fail(t, "no line")
		return ""
	}
}

func appendTo(t *testing.T, file, s string) {
	t.Helper()
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.WriteString(s)
	require.NoError(t, err)
}

func TestTailLogNotFollowed(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x.log")
	appendTo(t, file, "one\ntwo\nthree")
	ch, err := tailLog(context.Background(), file, false, 2)
	require.NoError(t, err)
	assert.Equal(t, "two\n", next(t, ch))
	assert.Equal(t, "three\n", next(t, ch), "a line with no end is sent at the end")
	_, ok := <-ch
	assert.False(t, ok)
}

func TestTailLogFollowed(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x.log")
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := tailLog(ctx, file, true, 100)
	require.NoError(t, err)

	appendTo(t, file, "created\n")
	assert.Equal(t, "created\n", next(t, ch), "the log to come is waited for")

	appendTo(t, file, "par")
	appendTo(t, file, "tial\n")
	assert.Equal(t, "partial\n", next(t, ch), "a line is sent when it ends")

	require.NoError(t, os.Rename(file, file+".0"))
	appendTo(t, file, "rotated\n")
	assert.Equal(t, "rotated\n", next(t, ch))

	require.NoError(t, os.Truncate(file, 0))
	appendTo(t, file, "x\n")
	assert.Equal(t, "x\n", next(t, ch), "truncated")

	cancel()
	select {
	case _, ok := <-ch:
		assert.False(t, ok)
	case <-time.After(5 * time.Second):
		require.Fail(t, "not closed")
	}
}

func TestTailLogFollowedFromTheEnd(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x.log")
	appendTo(t, file, "old\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := tailLog(ctx, file, true, 0)
	require.NoError(t, err)
	// The watcher is up before tailLog returns: this is either read as new
	// or, racing the first read, skipped as old with the rest of the file.
	time.Sleep(100 * time.Millisecond)
	appendTo(t, file, "new\n")
	assert.Equal(t, "new\n", next(t, ch))
}

func TestVmSpecSerialLog(t *testing.T) {
	s := vmSpec{name: "vm1", vcpus: 1, memoryMB: 512, osFlag: "--osinfo", osValue: "linux2024", serialLog: "/l/vm1.serial.log"}
	assert.Contains(t, s.args(), "pty,log.file=/l/vm1.serial.log,log.append=on")

	s.extra = []string{"--serial", "file,path=/x"}
	assert.NotContains(t, s.args(), "pty,log.file=/l/vm1.serial.log,log.append=on", "virtinst_args say")
}

func TestRemoveSerialLog(t *testing.T) {
	saved := serialLogDir
	serialLogDir = t.TempDir()
	defer func() { serialLogDir = saved }()
	r := &T{}
	r.Name = "vm1"
	require.NoError(t, r.removeSerialLog(), "none yet")
	appendTo(t, r.serialLogPath(), "said\n")
	require.NoError(t, r.removeSerialLog())
	assert.NoFileExists(t, r.serialLogPath())
}
