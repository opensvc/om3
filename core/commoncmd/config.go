package commoncmd

import (
	"bufio"
	"bytes"
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

	// Compile template regex once - match balanced braces
	templateRE := regexp.MustCompile(`\{[^{}]*\}`)

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
				color.Set(color.FgCyan).Fprint(out, key)

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

					// Highlight templates in the value
					templateMatches := templateRE.FindAllStringIndex(value, -1)
					if len(templateMatches) > 0 {
						lastPos := 0
						for _, match := range templateMatches {
							// Write non-template part
							out.WriteString(value[lastPos:match[0]])
							// Write template part in green + bold
							templateText := value[match[0]:match[1]]
							color.Set(color.FgGreen, color.Bold).Fprint(out, templateText)
							lastPos = match[1]
						}
						// Write remaining part after last template
						out.WriteString(value[lastPos:])
					} else {
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

		// Unmatched line - check for templates
		templateMatches := templateRE.FindAllStringIndex(line, -1)
		if len(templateMatches) > 0 {
			lastPos := 0
			for _, match := range templateMatches {
				// Write non-template part
				out.WriteString(line[lastPos:match[0]])
				// Write template part in green + bold
				templateText := line[match[0]:match[1]]
				color.Set(color.FgGreen, color.Bold).Fprint(out, templateText)
				lastPos = match[1]
			}
			// Write remaining part after last template
			out.WriteString(line[lastPos:])
			out.WriteString("\n")
		} else {
			// Unmatched line (output as-is)
			out.WriteString(line + "\n")
		}
	}

	return out.Bytes()
}
