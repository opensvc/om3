package client

type NamespaceType string
type Namespace struct{ value NamespaceType }

func (value NamespaceType) apply(s *Namespace) error { s.value = value; return nil }

type applyNamespace interface{ apply(*Namespace) error }

//goland:noinspection GoExportedFuncWithUnexportedType
func WithNamespace(value NamespaceType) applyNamespace { return value }
func (s Namespace) NamespaceValue() NamespaceType      { return s.value }
