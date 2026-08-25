package object

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/redact"
)

var (
	sectionRE  = regexp.MustCompile(`^\s*\[.*\]\s*$`)
	commentRE  = regexp.MustCompile(`^\s*[#;]`)
	keyValueRE = regexp.MustCompile(`^(\s*[^=\s#;]+)(\s*=\s*)([^#;]*)(.*)$`)
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

func RedactSecrets(b []byte, kind string) []byte {
	var (
		sections []redact.SectionData
		current  redact.SectionData
		secrets  map[redact.KeywordItem]bool
	)
	secrets = make(map[redact.KeywordItem]bool)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := scanner.Text()
		if sectionRE.MatchString(line) {
			if current.Name != "" && len(current.Lines) > 0 {
				sections = append(sections, current)
			}
			current = redact.SectionData{
				Name:   sectionName(line),
				Values: make(map[string]string),
			}
			current.Lines = append(current.Lines, redact.ParsedLine{Raw: line})
			continue
		}

		matches := keyValueRE.FindStringSubmatch(line)
		if len(matches) == 5 && !commentRE.MatchString(line) {
			k := strings.TrimSpace(matches[1])
			v := strings.TrimSpace(matches[3])

			current.Values[k] = v
			current.Lines = append(current.Lines, redact.ParsedLine{
				Raw:           line,
				IsKV:          true,
				KeyPrefix:     matches[1],
				Delim:         matches[2],
				Key:           k,
				InlineComment: matches[4],
			})
			continue
		}
		current.Lines = append(current.Lines, redact.ParsedLine{Raw: line})
	}
	sections = append(sections, current)

	k := naming.Kind(kind)
	var store keywords.Store
	if k == naming.KindInvalid {
		store = NodeKeywordStore
	} else {
		store = KeywordStoreWithDrivers(k)
	}

	for _, section := range sections {
		sectionType := section.Values["type"]
		dataSec := (k == naming.KindSec || k == naming.KindUsr) && section.Name == "data"
		for _, l := range section.Lines {
			kw := store.Lookup(key.New(section.Name, l.Key), k, sectionType)
			if dataSec || (kw != nil && kw.RedactSecret) {
				secrets[redact.KeywordItem{SectionName: section.Name, Key: l.Key}] = true
				continue
			}
		}
	}

	return redact.RedactSecrets(sections, secrets)
}
