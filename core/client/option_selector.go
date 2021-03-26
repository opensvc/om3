package client

type SelectorType string
type Selector struct{ value SelectorType }

func (value SelectorType) apply(s *Selector) error { s.value = value; return nil }

type applySelector interface{ apply(*Selector) error }

//goland:noinspection GoExportedFuncWithUnexportedType
func WithSelector(value string) applySelector  { return SelectorType(value) }
func (s Selector) SelectorValue() SelectorType { return s.value }
