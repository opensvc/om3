package redact

import (
	"bytes"
	"strings"
)

type (
	ParsedLine struct {
		Raw           string
		IsKV          bool
		Key           string
		KeyPrefix     string
		Delim         string
		InlineComment string
	}

	SectionData struct {
		Name   string
		Lines  []ParsedLine
		Values map[string]string
	}

	KeywordItem struct {
		SectionName string
		Key         string
	}
)

func sectionName(s string) string {
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

func RedactSecrets(sections []SectionData, secrets map[KeywordItem]bool) []byte {
	out := bytes.NewBuffer(nil)
	for _, section := range sections {
		for _, l := range section.Lines {
			if l.IsKV {
				if secret, ok := secrets[KeywordItem{SectionName: section.Name, Key: l.Key}]; ok && secret {
					out.WriteString(l.KeyPrefix)
					out.WriteString(l.Delim)
					out.WriteString("********")
					out.WriteString(l.InlineComment)
					out.WriteString("\n")
					continue
				}
			}
			out.WriteString(l.Raw)
			out.WriteString("\n")
		}
	}

	return out.Bytes()
}
