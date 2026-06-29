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

func (relations Relations) HasPath(path Path) (bool, error) {
	for _, relation := range relations {
		if p, err := relation.Path(); err != nil {
			return false, err
		} else if p == path {
			return true, nil
		}
	}
	return false, nil
}

func (relations Relations) Strings() []string {
	l := make([]string, len(relations))
	for i, relation := range relations {
		l[i] = string(relation)
	}
	return l
}

// ParseRelations resolves test/svc/svc1 relations with a local-centric
// policy.
//
// Explicitely local:
//
//	children = ./svc/svc2    => test/svc/svc2
//
// Implicitely local:
//
//	children = svc3          => test/svc/svc3
//
// Explicitely foreign:
//
//	children = root/svc/svc2 => root/svc/svc2
//
// Implicitely local with scope:
//
//	children svc4@n1         => test/svc/svc4@n1
func ParseRelations(l []string, ns string) Relations {
	relations := make(Relations, 0)
	for _, s := range l {
		if !strings.Contains(s, "/") {
			// implicitely local
			s = "./svc/" + s
		}
		s, node, ok := strings.Cut(s, "@")
		path, err := ParsePathRel(s, ns)
		if err != nil {
			continue
		}
		if ok {
			s = path.String() + "@" + node
		} else {
			s = path.String()
		}
		relations = append(relations, Relation(s))
	}
	return relations
}
