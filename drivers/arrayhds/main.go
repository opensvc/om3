// Package arrayhds drives a Hitachi array through the HiCommand Device
// Manager command line interface.
//
// It is a port of the v2 agent driver, and answers to the same commands with
// the same options, because the collector drives an array by running them.
// The collector calls this driver for its hds, hm700 and r700 models alike:
// they are one array type, named three ways.
package arrayhds

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// Array is one Hitachi array, as the node or cluster configuration
	// declares it.
	Array struct {
		array.Array
	}
)

// jrePathEnv is where the HiCommand cli looks for the java runtime.
const jrePathEnv = "HDVM_CLI_JRE_PATH"

func init() {
	driver.Register(driver.NewID(driver.GroupArray, "hds"), NewDriver)
}

// NewDriver returns a Hitachi array driver.
func NewDriver() array.Driver {
	t := New()
	var i any = t
	return i.(array.Driver)
}

// New returns a Hitachi array.
func New() *Array {
	return &Array{}
}

// Run builds the command tree of this array and runs the arguments through
// it. What the tree holds is declared in Actions.
func (t *Array) Run(args []string) error {
	return array.RunActions(context.Background(), t.Actions(), args, os.Stdout)
}

// arrayName is the name the array is known by, which the model and the serial
// are read from.
func (t Array) arrayName() string {
	if s := t.Config().GetString(t.Key("name")); s != "" {
		return s
	}
	return strings.TrimPrefix(t.Name(), "array#")
}

// model is the first part of the array name, "HUS VM" in "HUS VM.210945".
func (t Array) model() string {
	name := t.arrayName()
	if i := strings.Index(name, "."); i >= 0 {
		return name[:i]
	}
	return name
}

// serial is the last part of the array name.
func (t Array) serial() string {
	name := t.arrayName()
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func (t Array) bin() string {
	if s := t.Config().GetString(t.Key("bin")); s != "" {
		return s
	}
	return "HiCommandCLI"
}

func (t Array) jrePath() string {
	return t.Config().GetString(t.Key("jre_path"))
}

func (t Array) url() string {
	return t.Config().GetString(t.Key("url"))
}

func (t Array) username() string {
	return t.Config().GetString(t.Key("username"))
}

// password returns the password the manager authenticates with.
//
// The keyword names a key of a datastore rather than holding the password, so
// the password of an array is not written in a configuration every node reads.
func (t Array) password() (string, error) {
	s, err := t.Config().GetStringStrict(t.Key("password"))
	if err != nil {
		return "", err
	}
	km, err := datarecv.ParseKeyMetaRelWithFallback(s, naming.NsSys, "password")
	if err != nil {
		return "", err
	}
	b, err := km.RootDecode()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (t Array) log() *plog.Logger {
	return plog.NewDefaultLogger().WithPrefix("array: "+t.Name()+": ").Attr("array", t.Name())
}

// run runs one manager command and returns what it printed.
//
// A command is scoped to this array unless it names its own scope, and asks
// for xml when what it returns is read as a tree rather than as the indented
// text the manager answers a change with.
func (t *Array) run(ctx context.Context, asXML, scoped bool, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("%s: no command", t.Name())
	}
	if t.url() == "" {
		return "", fmt.Errorf("%s: the url keyword is required", t.Name())
	}
	password, err := t.password()
	if err != nil {
		return "", fmt.Errorf("%s: password: %w", t.Name(), err)
	}

	l := []string{t.url(), args[0], "-u", t.username(), "-p", password}
	if asXML {
		l = append(l, "-f", "xml")
	}
	if scoped {
		l = append(l, "serialnum="+t.serial(), "model="+t.model())
	}
	l = append(l, args[1:]...)

	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(t.bin()),
		command.WithArgs(l),
		command.WithLogger(t.log()),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
		command.WithStderrLogLevel(zerolog.TraceLevel),
		command.WithVarEnv(t.env()...),
	)
	b, err := cmd.Output()
	if err != nil {
		return string(b), fmt.Errorf("%s: %s: %w", t.Name(), args[0], err)
	}
	return string(b), nil
}

// env is what the manager cli needs in its environment.
func (t Array) env() []string {
	l := make([]string, 0)
	if s := t.jrePath(); s != "" {
		l = append(l, jrePathEnv+"="+s)
	}
	return l
}

// toDevnum returns the device number the manager reads, from any of the ways a
// device is named elsewhere.
//
// A device is written "00:00:00" or "00:00" on the array, "<serial>.<culd>" in
// the inventory of the collector, and as a 32 or 33 character wwid by a host.
// All three are hexadecimal, and the manager wants a decimal number.
func toDevnum(devnum string) string {
	switch {
	case strings.Contains(devnum, ":"):
		return fromHex(strings.ReplaceAll(devnum, ":", ""), devnum)
	case strings.Contains(devnum, "."):
		l := strings.Split(devnum, ".")
		return fromHex(l[len(l)-1], devnum)
	case len(devnum) == 32 || len(devnum) == 33:
		return fromHex(devnum[len(devnum)-4:], devnum)
	default:
		return devnum
	}
}

// fromHex returns the decimal form of a hexadecimal number, or the value as it
// was written when it is not one.
func fromHex(s, orig string) string {
	i, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return orig
	}
	return strconv.FormatInt(i, 10)
}

// parse reads what the manager answers a change with.
//
// The answer is not the xml a query answers. It is a list of instances, each
// opened by a line naming its type and followed by its key=value lines,
// indented under it. An instance may hold a list of its own, opened by a line
// reading "List of <n> <Type> elements".
//
// A value that is a number is read as one, commas and spaces included, as the
// manager writes a capacity that way.
func parse(out string) []map[string]any {
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "RESPONSE:" {
		// The first line names the answer rather than saying anything.
		lines = lines[1:]
	}
	// Trailing blank lines would be read as the end of everything.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return []map[string]any{}
	}
	l, _ := parseList(lines, 0)
	return l
}

// parseList reads the instances a marker line repeats, and returns where it
// stopped.
func parseList(lines []string, start int) ([]map[string]any, int) {
	l := make([]map[string]any, 0)
	if start >= len(lines) {
		return l, start
	}
	refIndent := indentOf(lines[start])
	marker := lines[start]
	i := start
	for i < len(lines) {
		indent := indentOf(lines[i])
		if indent < refIndent {
			return l, i
		}
		if indent > refIndent {
			i++
			continue
		}
		if lines[i] != marker {
			i++
			continue
		}
		instance, next := parseInstance(lines, i+1, indent)
		l = append(l, instance)
		i = next
	}
	return l, i
}

// parseInstance reads the keys of one instance, and the lists it holds.
func parseInstance(lines []string, start, refIndent int) (map[string]any, int) {
	data := make(map[string]any)
	i := start
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}
		if indentOf(line) <= refIndent {
			return data, i
		}
		if name, ok := listHeader(line); ok {
			nested, next := parseList(lines, i+1)
			data[name] = nested
			i = next
			continue
		}
		if key, value, ok := keyValue(line); ok {
			data[key] = value
		}
		i++
	}
	return data, i
}

// listHeader reads a "List of <n> <Type> elements" line and returns the type
// the list holds.
func listHeader(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 4 || fields[0] != "List" || fields[1] != "of" {
		return "", false
	}
	return fields[3], true
}

// indentOf returns how far a line is indented.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// keyValue reads a "key=value" line.
func keyValue(line string) (string, any, bool) {
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", nil, false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return "", nil, false
	}
	if i, err := strconv.ParseInt(strings.NewReplacer(" ", "", ",", "").Replace(value), 10, 64); err == nil {
		return key, i, true
	}
	return key, value, true
}

// unmarshalXML reads the tree the manager answers a query with.
func unmarshalXML(out string, v any) error {
	return xml.Unmarshal([]byte(out), v)
}
