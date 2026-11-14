package mdmon

import (
	"bufio"
	"bytes"
	"regexp"
	"slices"
	"strings"
)

var (
	// mdstatHeader matches the first line of the block of an array, as
	// "md127 : active raid1 sdb[1] sda[0]".
	mdstatHeader = regexp.MustCompile(`^(md\S*)\s*:\s*(.*)$`)

	// mdstatAction matches the progress line of a resync, a recovery, a
	// reshape or a check of an array, as "[=>....]  resync =  8.5% (...)"
	// or "resync=DELAYED".
	mdstatAction = regexp.MustCompile(`\b(resync|recovery|reshape|check|repair)\s*=\s*(DELAYED|PENDING)?`)
)

// parseMdstat returns the signature of each array /proc/mdstat lists, by its
// kernel name: the lines of its block, but for what changes with no md event,
// the progress of an action, reduced to its kind, and the bitmap usage. Two
// signatures differ when the array started or stopped, changed level, a
// member failed, was added or removed, or an action started or ended.
func parseMdstat(b []byte) map[string]string {
	arrays := make(map[string]string)
	var name string
	var sig []string
	flush := func() {
		if name != "" {
			arrays[name] = strings.Join(sig, "\n")
		}
		name, sig = "", nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if m := mdstatHeader.FindStringSubmatch(line); m != nil {
			flush()
			name = m[1]
			sig = []string{m[2]}
			continue
		}
		if name == "" {
			continue
		}
		switch {
		case trimmed == "":
			flush()
		case strings.HasPrefix(trimmed, "bitmap:"):
		case mdstatAction.MatchString(trimmed):
			m := mdstatAction.FindStringSubmatch(trimmed)
			sig = append(sig, strings.TrimSpace(m[1]+" "+m[2]))
		default:
			sig = append(sig, trimmed)
		}
	}
	flush()
	return arrays
}

// changedArrays returns the kernel names of the arrays whose signature
// differs between prev and cur, or which are in one of them only, sorted.
func changedArrays(prev, cur map[string]string) []string {
	l := make([]string, 0)
	for name, sig := range cur {
		if was, ok := prev[name]; !ok || was != sig {
			l = append(l, name)
		}
	}
	for name := range prev {
		if _, ok := cur[name]; !ok {
			l = append(l, name)
		}
	}
	slices.Sort(l)
	return l
}
