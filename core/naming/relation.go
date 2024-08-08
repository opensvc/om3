package naming

import "strings"

type (
	// Relation is an object path or an instance path (path@node).
	Relation string

	// Relations is a slice of Relation
	Relations []Relation
)

func (t Relation) String() string {
	return string(t)
}

func (t Relation) Split() (Path, string, error) {
	p, err := t.Path()
	return p, t.Node(), err
}

func (t Relation) Node() string {
	var s string
	if strings.Contains(string(t), "@") {
		s = strings.SplitN(string(t), "@", 2)[1]
	}
	return s
}

func (t Relation) Path() (Path, error) {
	var s string
	if strings.Contains(string(t), "@") {
		s = strings.SplitN(string(t), "@", 2)[0]
	} else {
		s = string(t)
	}
	return ParsePath(s)
}

func (relations Relations) Strings() []string {
	l := make([]string, len(relations))
	for i, relation := range relations {
		l[i] = string(relation)
	}
	return l
}

func ParseRelations(l []string) Relations {
	relations := make(Relations, len(l))
	for i, s := range l {
		relations[i] = Relation(s)
	}
	return relations
}
