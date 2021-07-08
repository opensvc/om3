package object

import (
	"strings"
)

func (t *Node) ListPools() []string {
	l := make([]string, 0)
	for _, s := range t.MergedConfig().SectionStrings() {
		if !strings.HasPrefix(s, "pool#") {
			continue
		}
		l = append(l, s[5:])
	}
	return l
}
