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
	"fmt"
	"os"
	"strconv"
	"strings"

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

		// cli runs the manager command line and returns what it wrote. It
		// is runCLI but in the tests, which answer as a manager would without
		// an array behind it.
		cli func(ctx context.Context, name string, args, env []string) (stdout, stderr []byte, err error)

		// secret returns the password the manager authenticates with. It is
		// the password method but in the tests, which have no datastore.
		secret func() (string, error)

		// journal is what an action returns under its "log" key, as v2 did:
		// each command changing the array, its password masked, followed by
		// the lines it wrote, stdout lines at level 0 and stderr lines at
		// level 1. The collector prints them in the output of the form.
		journal []any
	}
)

// maxOutput is how much of what the manager wrote an error quotes, per
// stream: enough to read its message, not a whole xml tree.
const maxOutput = 4096

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
//
// The command line is logged with its password masked, and the command is
// not handed the logger: the command package would log the line unmasked.
// A command changing the array, which is every command not asking for xml,
// is recorded in the journal.
//
// An error quotes what the manager wrote on both streams, where it says why
// it refused.
func (t *Array) run(ctx context.Context, asXML, scoped bool, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("%s: no command", t.Name())
	}
	if t.url() == "" {
		return "", fmt.Errorf("%s: the url keyword is required", t.Name())
	}
	secret := t.secret
	if secret == nil {
		secret = t.password
	}
	password, err := secret()
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

	masked := append([]string{t.bin()}, l...)
	masked[6] = "xxxx"
	line := strings.Join(masked, " ")
	t.log().Debugf("run %s", line)

	cli := t.cli
	if cli == nil {
		cli = runCLI
	}
	stdout, stderr, err := cli(ctx, t.bin(), l, t.env())
	if !asXML {
		t.record(line, stdout, stderr)
	}
	if err != nil {
		return string(stdout), fmt.Errorf("%s: %s: %w%s", t.Name(), strings.Join(args, " "), err, outputOf(stdout, stderr))
	}
	return string(stdout), nil
}

// runCLI runs the manager command line.
func runCLI(ctx context.Context, name string, args, env []string) ([]byte, []byte, error) {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(name),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithVarEnv(env...),
	)
	err := cmd.Run()
	return cmd.Stdout(), cmd.Stderr(), err
}

// record adds a command and what it wrote to the journal.
func (t *Array) record(line string, stdout, stderr []byte) {
	t.journal = append(t.journal, []any{0, line, map[string]any{}})
	for _, s := range lines(stdout) {
		t.journal = append(t.journal, []any{0, s, map[string]any{}})
	}
	for _, s := range lines(stderr) {
		t.journal = append(t.journal, []any{1, s, map[string]any{}})
	}
}

// logEntries returns the journal, never nil, so an action reports an empty list
// rather than a null.
func (t *Array) logEntries() []any {
	if t.journal == nil {
		return []any{}
	}
	return t.journal
}

// lines returns the lines of an output, as v2 split them.
func lines(b []byte) []string {
	s := strings.TrimSpace(strings.ReplaceAll(string(b), "\r\n", "\n"))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// outputOf renders what a command wrote, for an error to quote.
func outputOf(stdout, stderr []byte) string {
	var b strings.Builder
	for _, stream := range []struct {
		name string
		data []byte
	}{{"stderr", stderr}, {"stdout", stdout}} {
		s := strings.TrimSpace(string(stream.data))
		if s == "" {
			continue
		}
		if len(s) > maxOutput {
			s = s[:maxOutput] + "..."
		}
		fmt.Fprintf(&b, "\n%s: %s", stream.name, s)
	}
	return b.String()
}

// env is what the manager cli needs in its environment.
func (t Array) env() []string {
	l := make([]string, 0)
	if s := t.jrePath(); s != "" {
		l = append(l, jrePathEnv+"="+s)
	}
	return l
}

// toDevnum returns the decimal device number the manager reads, from the
// ways a device is written with no array to ask.
//
// The array writes a device "00:00:64" or "00:64", its LDKC, CU and LDEV in
// hexadecimal, and a host writes it as the 32 or 33 character wwid ending
// with the CU and LDEV. A decimal number is the device number itself.
//
// What is none of these is refused rather than passed to the manager, as v2
// refused the forms it could not read: a device number misread is another
// client's volume deleted. Leading zeros on a number are refused for that
// reason: "0064" is hexadecimal where the array writes it, decimal where the
// manager reads it, and the two are not the same volume.
//
// A "<serial>.<n>" disk id is not read here: what its last part counts is
// the object id of the volume, which devNumOf asks the array for.
func toDevnum(devnum string) (string, error) {
	invalid := func(why string) (string, error) {
		return "", fmt.Errorf("devnum %q: %s: write it as the array does, 00:00:64, as a disk id, <serial>.<n>, as a 32 or 33 character wwid, or as a decimal number", devnum, why)
	}
	switch {
	case devnum == "":
		return invalid("empty")
	case strings.Contains(devnum, "."):
		return invalid("a disk id is resolved through the array")
	case strings.Contains(devnum, ":"):
		groups := strings.Split(devnum, ":")
		if len(groups) != 2 && len(groups) != 3 {
			return invalid("not two or three groups of two hexadecimal digits")
		}
		for _, group := range groups {
			if len(group) != 2 || !isHex(group) {
				return invalid("not two or three groups of two hexadecimal digits")
			}
		}
		return fromHex(strings.Join(groups, ""))
	case isWWID(devnum):
		return invalid("a wwid is resolved through the array")
	case isDecimal(devnum):
		return devnum, nil
	default:
		return invalid("not a device number")
	}
}

// isWWID reports whether a device is named by the wwid a host sees it as: 32
// hexadecimal digits, or 33 with the leading "3" of the scsi naa type.
func isWWID(s string) bool {
	return (len(s) == 32 || len(s) == 33) && isHex(s)
}

// devNumOfWWID returns the device number of the volume of this array a host
// sees as wwid, as the collector matches a wwid to a volume.
//
// The wwid is "60" followed by characters 2 to 12 of the world wide names of
// the ports of its array, and ends with the 6 hexadecimal digits of the
// display name of the volume, its ldkc, cu and ldev. A wwid of another array
// is refused, rather than read as the device number of a volume of this one,
// and so is a wwid whose volume is not listed with the display name it ends
// with: v2 read its last 4 digits alone, which dropped the ldkc and acted on
// the volume of another ldkc, or of another array, with the same cu and ldev.
func (t *Array) devNumOfWWID(ctx context.Context, wwid string) (string, error) {
	w := strings.ToLower(wwid)
	if len(w) == 33 {
		if w[0] != '3' {
			return "", fmt.Errorf("devnum %q: a 33 character wwid starts with 3", wwid)
		}
		w = w[1:]
	}
	if !strings.HasPrefix(w, "60") {
		return "", fmt.Errorf("devnum %q: not the wwid of a volume of a hitachi array, which starts with 60", wwid)
	}
	ports, err := t.getPorts(ctx)
	if err != nil {
		return "", err
	}
	var ofThisArray bool
	for _, p := range ports {
		wwpn := strings.ReplaceAll(p.WWPN, ":", "")
		if len(wwpn) >= 12 && wwpn[2:12] == w[2:12] {
			ofThisArray = true
			break
		}
	}
	if !ofThisArray {
		return "", fmt.Errorf("devnum %q: the wwid is not of array %s (serial %s): no port of its %d has the world wide name it is made of", wwid, t.arrayName(), t.serial(), len(ports))
	}
	ldev := w[26:]
	devNum, err := fromHex(ldev)
	if err != nil {
		return "", err
	}
	units, err := t.getLogicalUnits(ctx, "")
	if err != nil {
		return "", err
	}
	unit, err := unitByDevNum(units, devNum)
	if err != nil {
		return "", fmt.Errorf("devnum %q: %w", wwid, err)
	}
	if name := strings.ToLower(strings.ReplaceAll(unit.DisplayName, ":", "")); name != ldev {
		return "", fmt.Errorf("devnum %q: the volume of device number %s is listed as %s, not as the %s the wwid ends with", wwid, devNum, unit.DisplayName, ldev)
	}
	return devNum, nil
}

// isDecimal reports whether a string is a decimal number written with no
// leading zero, which is how the manager writes a device number.
func isDecimal(s string) bool {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// fromHex returns the decimal form of a hexadecimal number.
func fromHex(s string) (string, error) {
	i, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return "", fmt.Errorf("devnum %q: %w", s, err)
	}
	return strconv.FormatInt(i, 10), nil
}

// devNumOf returns the decimal device number of a device, written any of the
// ways toDevnum reads, or as the "<serial>.<n>" disk id "add disk" answers.
//
// A disk id is the last two parts of the object id of the volume. Whether
// its last part counts in decimal or in hexadecimal is the manager's
// business, so it is not converted: the volume is the one of this array
// whose object id ends with it, and a disk id no volume or several volumes
// end with is an error.
func (t *Array) devNumOf(ctx context.Context, devnum string) (string, error) {
	if isWWID(devnum) {
		return t.devNumOfWWID(ctx, devnum)
	}
	if !strings.Contains(devnum, ".") {
		return toDevnum(devnum)
	}
	units, err := t.getLogicalUnits(ctx, "")
	if err != nil {
		return "", err
	}
	var found []logicalUnit
	for _, unit := range units {
		if unit.diskID() == devnum {
			found = append(found, unit)
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("devnum %q: no volume of array %s (serial %s) has this disk id, among the %d it lists", devnum, t.arrayName(), t.serial(), len(units))
	case 1:
	default:
		return "", fmt.Errorf("devnum %q: %d volumes of array %s have this disk id", devnum, len(found), t.arrayName())
	}
	if !isDecimal(found[0].DevNum) {
		return "", fmt.Errorf("devnum %q: the volume of this disk id has the device number %q, which is not a number", devnum, found[0].DevNum)
	}
	return found[0].DevNum, nil
}

// textKeys are the keys of an answer kept as text whatever they hold. They
// name a volume, and a name read as a number loses its leading zeros: a
// display name of "0064" is not the volume of a display name of "64".
var textKeys = map[string]bool{
	"displayName": true,
	"objectID":    true,
	"label":       true,
	"name":        true,
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
	for i, line := range lines {
		if strings.TrimSpace(line) == "RESPONSE:" {
			// The line names the answer rather than saying anything, and
			// what comes before it, as a blank line or a warning, is not
			// the answer either.
			lines = lines[i+1:]
			break
		}
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
	if textKeys[key] {
		return key, value, true
	}
	if i, err := strconv.ParseInt(strings.NewReplacer(" ", "", ",", "").Replace(value), 10, 64); err == nil {
		return key, i, true
	}
	return key, value, true
}
