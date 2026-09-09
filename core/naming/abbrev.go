package naming

import "strings"

// abbrevMaxLabels is how many labels deep the common suffix is looked for.
// A name is not expected to be deeper, and the search has to stop somewhere.
const abbrevMaxLabels = 10

// Abbrev shortens fully qualified names to the part that tells them apart,
// for the column headers of a listing whose width the names would otherwise
// decide.
//
// The domain the names share carries no information once they are shown side
// by side: in a cluster of node1.prod.example.com and node2.prod.example.com,
// only the first label differs, and the rest costs eighteen columns per node
// to say the same thing twice. What is dropped is replaced by a trailing "..",
// so a shortened name is never mistaken for a name that is short.
//
// The domain the names do not share is kept, because that is the part telling
// them apart: node1.paris.example.com and node1.lyon.example.com abbreviate to
// node1.paris.. and node1.lyon.., not to two identical node1.. columns.
//
// A name with no domain is left alone, and so are names that already differ at
// their last label, which have no shared suffix to drop.
//
// This is the abbrev() of OpenSVC v2, ported so the two agents render a
// cluster the same way.
func Abbrev(names []string) []string {
	if len(names) < 1 {
		return names
	}

	// A name is compared from its last label inward, so it is held reversed:
	// the domain the names may share is then a common prefix.
	paths := make([][]string, len(names))
	for i, name := range names {
		paths[i] = reversedLabels(name)
	}

	// Only a name holding a domain has anything to drop.
	trimable := make([][]string, 0, len(paths))
	for _, path := range paths {
		if len(path) > 1 {
			trimable = append(trimable, path)
		}
	}
	if len(trimable) <= 1 {
		// There is no second domain to compare against, so nothing in a
		// domain can be telling names apart, and all of it goes.
		l := make([]string, len(paths))
		for i, path := range paths {
			if len(path) > 1 {
				l[i] = path[len(path)-1] + ".."
			} else {
				l[i] = path[0]
			}
		}
		return l
	}

	// The first label the names disagree on ends the domain they share.
	depth := 0
	for i := 0; i < abbrevMaxLabels; i++ {
		depth = i
		labels := make(map[string]bool, len(trimable))
		short := false
		for _, path := range trimable {
			if i >= len(path) {
				short = true
				break
			}
			labels[path[i]] = true
		}
		if short || len(labels) > 1 {
			break
		}
	}
	if depth == 0 {
		// The names differ at their last label already, so every label of
		// every name is telling them apart.
		return names
	}
	return abbreviated(paths, depth)
}

// abbreviated renders reversed label lists, keeping the labels above depth and
// marking what was dropped, and leaving a name with no domain untouched.
func abbreviated(paths [][]string, depth int) []string {
	l := make([]string, len(paths))
	for i, path := range paths {
		if len(path) < 2 {
			l[i] = path[0]
			continue
		}
		kept := make([]string, 0, len(path))
		for j := len(path) - 1; j >= depth; j-- {
			kept = append(kept, path[j])
		}
		l[i] = strings.Join(kept, ".") + ".."
	}
	return l
}

// reversedLabels splits a name on its dots, last label first.
func reversedLabels(name string) []string {
	labels := strings.Split(name, ".")
	l := make([]string, len(labels))
	for i, label := range labels {
		l[len(labels)-1-i] = label
	}
	return l
}
