package commoncmd

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/iancoleman/orderedmap"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/key"
)

func APIKeywordItemsToRaw(items api.KeywordItems) rawconfig.T {
	r := rawconfig.T{}
	r.Data = orderedmap.New()

	for _, item := range items {
		k := key.Parse(item.Keyword)
		i, ok := r.Data.Get(k.Section)
		var sectionMap orderedmap.OrderedMap
		if ok {
			sectionMap = i.(orderedmap.OrderedMap)
		} else {
			sectionMap = *orderedmap.New()
		}
		sectionMap.Set(k.Option, item.Value)
		r.Data.Set(k.Section, sectionMap)
	}
	return r
}

var (
	sectionRE = regexp.MustCompile(`^\s*\[.*\]\s*$`)
	commentRE = regexp.MustCompile(`^\s*[#;]`)
)

func Sections(b []byte, sections []string) []byte {
	if len(sections) == 0 {
		return b
	}
	out := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	var inValidSection bool
	m := make(map[string]any)
	for _, section := range sections {
		m[section] = nil
	}
	sectionName := func(s string) string {
		s = strings.TrimSpace(s)
		if !strings.HasPrefix(s, "[") {
			return ""
		}
		if !strings.HasSuffix(s, "]") {
			return ""
		}
		s = s[1 : len(s)-1]
		s = strings.TrimSpace(s)
		return s
	}
	isValidSection := func(s string) bool {
		_, ok := m[s]
		return ok
	}
	for scanner.Scan() {
		line := scanner.Text()
		s := sectionName(line)
		if s == "" {
			if inValidSection {
				out.WriteString(line + "\n")
			}
		} else {
			inValidSection = isValidSection(s)
			if inValidSection {
				out.WriteString(line + "\n")
			}
		}
	}
	return out.Bytes()
}

func ColorizeINI(b []byte) []byte {
	if color.NoColor {
		return b
	}
	out := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	var continuedValue string
	var continuation bool

	for scanner.Scan() {
		line := scanner.Text()

		// Handle continuation lines
		if continuation {
			continuedValue += "\n" + line
			if strings.HasSuffix(strings.TrimRight(line, " \t"), `\`) {
				continue
			}
			out.WriteString(continuedValue + "\n")
			continuedValue = ""
			continuation = false
			continue
		}

		// Section header
		if sectionRE.MatchString(line) {
			color.Set(color.FgYellow).Fprintln(out, line)
			continue
		}

		// Comment
		if commentRE.MatchString(line) {
			color.Set(color.FgHiBlack).Fprintln(out, line)
			continue
		}

		// Key-value
		if strings.Contains(line, "=") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := parts[1] // preserve spacing
			color.Set(color.FgMagenta).Fprint(out, key)
			out.WriteString(fmt.Sprintf(" =%s\n", value))

			// Check if line continues
			if strings.HasSuffix(strings.TrimRight(value, " \t"), `\`) {
				continuedValue = ""
				continuedValue += value
				continuation = true
			}
			continue
		}

		// Unmatched line (output as-is)
		out.WriteString(line + "\n")
	}

	return out.Bytes()
}
