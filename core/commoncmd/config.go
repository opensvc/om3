package commoncmd

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/iancoleman/orderedmap"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/key"
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

func Sections(b []byte, sections []string) ([]byte, error) {
	if len(sections) == 0 {
		return b, nil
	}

	requested := make(map[string]bool)
	for _, sec := range sections {
		requested[sec] = false
	}

	scanner := bufio.NewScanner(bytes.NewReader(b))
	sectionName := func(s string) string {
		s = strings.TrimSpace(s)
		if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
			return ""
		}
		return strings.TrimSpace(s[1 : len(s)-1])
	}

	for scanner.Scan() {
		line := scanner.Text()
		if name := sectionName(line); name != "" {
			if _, found := requested[name]; found {
				requested[name] = true
			}
		}
	}

	for sec, found := range requested {
		if !found {
			return nil, fmt.Errorf("no such section %s", sec)
		}
	}

	out := bytes.NewBuffer(nil)
	scanner = bufio.NewScanner(bytes.NewReader(b))
	var inValidSection bool
	m := make(map[string]any)
	for _, section := range sections {
		m[section] = nil
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
	return out.Bytes(), nil
}

func ColorizeINI(b []byte) []byte {
	if color.NoColor {
		return b
	}
	out := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	var continuedValue string
	var continuation bool

	// Compile regexes once
	referenceRE := regexp.MustCompile(`\{[^{}]*\}`)

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
			color.Set(color.FgHiYellow, color.Bold).Fprintln(out, line)
			continue
		}

		// Comment
		if commentRE.MatchString(line) {
			color.Set(color.FgHiBlack, color.Italic).Fprintln(out, line)
			continue
		}

		// Key-value
		if strings.Contains(line, "=") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// Use regex to preserve spacing around equals sign
			kvRE := regexp.MustCompile(`^(\s*[^=\s]+(?:\s+[^=\s]+)*)\s*=\s*(.*)$`)
			matches := kvRE.FindStringSubmatch(line)
			if len(matches) == 3 {
				key := matches[1]
				equalAndValue := matches[2]

				// Colorize key
				key, scope, scopeFound := strings.Cut(key, "@")
				color.Set(color.FgCyan).Fprint(out, key)
				if scopeFound {
					color.Set(color.FgHiMagenta).Fprint(out, "@"+scope)
				}

				// Find the equals sign position to preserve exact spacing
				equalPos := strings.Index(line, "=")
				if equalPos >= 0 {
					// Extract the equals sign with surrounding spaces
					start := equalPos
					end := equalPos + 1
					// Include leading spaces
					for start > 0 && line[start-1] == ' ' {
						start--
					}
					// Include trailing spaces
					for end < len(line) && line[end] == ' ' {
						end++
					}
					equalSign := line[start:end]
					color.Set(color.FgHiBlack).Fprint(out, equalSign)

					// The rest is the value
					value := line[end:]

					// Highlight references in the value
					referenceMatches := referenceRE.FindAllStringIndex(value, -1)
					if len(referenceMatches) > 0 {
						lastPos := 0
						for _, match := range referenceMatches {
							// Write non-reference part
							out.WriteString(value[lastPos:match[0]])

							// Write reference part in green + bold
							referenceText := value[match[0]:match[1]]
							color.Set(color.FgGreen, color.Bold).Fprint(out, referenceText)
							lastPos = match[1]
						}
						// Write remaining part after last reference
						out.WriteString(value[lastPos:])
					} else {
						// No references
						out.WriteString(value)
					}
				} else {
					// Fallback: output the rest as-is
					out.WriteString(equalAndValue)
				}
				out.WriteString("\n")

				// Check if line continues
				if strings.HasSuffix(strings.TrimRight(line, " \t"), `\`) {
					continuedValue = ""
					continuedValue += line
					continuation = true
				}
				continue
			}
		}

		// Unmatched line - check for references
		referenceMatches := referenceRE.FindAllStringIndex(line, -1)
		if len(referenceMatches) > 0 {
			lastPos := 0
			for _, match := range referenceMatches {
				// Write non-reference part
				out.WriteString(line[lastPos:match[0]])

				// Write reference part in green + bold
				referenceText := line[match[0]:match[1]]
				color.Set(color.FgGreen, color.Bold).Fprint(out, referenceText)
				lastPos = match[1]
			}
			// Write remaining part after last reference
			out.WriteString(line[lastPos:])
			out.WriteString("\n")
		} else {
			// Unmatched line (output as-is)
			out.WriteString(line)
			out.WriteString("\n")
		}
	}

	return out.Bytes()
}
