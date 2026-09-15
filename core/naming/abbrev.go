package naming

import "strings"

// Abbrev shortens fully qualified names to the labels that tell them apart,
// for the column headers of a listing whose width the names would otherwise
// decide.
//
// A label every name carries the same value at carries no information once
// the names are shown side by side, wherever in the name it sits. In a cluster
// of node1.prod.example.com and node2.prod.example.com only the first label
// differs, and the rest costs eighteen columns per node to say the same thing
// three times. Dropped labels are replaced by "..", so a shortened name is
// never mistaken for a name that is short, and so the reader can see that
// something stood where the marker is:
//
//	ip-xxx-xxx-xxx.commondomain1.eu-fr-paris.commondomain2.etc
//	ip-yyy-yyy-zzz.commondomain1.eu-fr-north.commondomain2.etc
//
// abbreviates to
//
//	ip-xxx-xxx-xxx..eu-fr-paris..
//	ip-yyy-yyy-zzz..eu-fr-north..
//
// A run of dropped labels is one marker, not one per label: the marker says
// that a name was cut here, and how many labels were cut is not something a
// column header is the place to count.
//
// The labels the names do not share are kept, because they are what tells them
// apart: node1.paris.example.com and node1.lyon.example.com abbreviate to
// node1.paris.. and node1.lyon.., not to two identical node1.. columns.
//
// The first label is kept even when every name shares it, because it is the
// name of the node, and a column headed "..paris.." names nothing an operator
// can act on.
//
// Names are compared right to left, which is how a domain name is anchored: a
// position then means the same thing in every name, whatever their depths. A
// name with no domain has nothing to drop, and does not get a say in what the
// others share.
//
// This generalises the abbrev() of OpenSVC v2, which dropped only the shared
// tail. v2 returns a.example.com and b.example.org untouched, having no shared
// tail to drop; this returns a..com and b..org, the shared label in the middle
// being no more informative for sitting there.
func Abbrev(names []string) []string {
	if len(names) == 0 {
		return names
	}

	paths := make([][]string, len(names))
	depth := 0
	for i, name := range names {
		paths[i] = reversedLabels(name)
		if len(paths[i]) > depth {
			depth = len(paths[i])
		}
	}

	// Only a name holding a domain has anything to drop, and only such a name
	// can say whether a label is shared: a bare hostname beside a qualified
	// one must not make the qualified one's domain look unshared.
	qualified := make([][]string, 0, len(paths))
	for _, path := range paths {
		if len(path) > 1 {
			qualified = append(qualified, path)
		}
	}

	shared := sharedLabels(qualified, depth)
	l := make([]string, len(paths))
	for i, path := range paths {
		l[i] = abbreviated(path, shared)
	}
	return l
}

// sharedLabels reports, for each position, whether every name carries the same
// label there. A name too short to reach a position shares nothing there.
func sharedLabels(paths [][]string, depth int) []bool {
	shared := make([]bool, depth)
	if len(paths) == 0 {
		return shared
	}
	for i := 0; i < depth; i++ {
		same := i < len(paths[0])
		for _, path := range paths[1:] {
			if i >= len(path) || path[i] != paths[0][i] {
				same = false
				break
			}
		}
		shared[i] = same
	}
	return shared
}

// abbreviated renders one reversed label list, keeping the labels the names do
// not share and collapsing each run of the ones they do into a single marker.
func abbreviated(path []string, shared []bool) string {
	var (
		b         strings.Builder
		inRun     bool
		afterName bool
	)
	for i := len(path) - 1; i >= 0; i-- {
		keep := i == len(path)-1 || i >= len(shared) || !shared[i]
		if keep {
			if afterName {
				// Two kept labels in a row are still separated by a dot. A
				// marker separates by itself.
				b.WriteString(".")
			}
			b.WriteString(path[i])
			afterName = true
			inRun = false
			continue
		}
		if !inRun {
			b.WriteString("..")
			afterName = false
			inRun = true
		}
	}
	return b.String()
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
