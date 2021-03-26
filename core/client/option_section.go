package client

type SectionType string
type Section struct{ value SectionType }

func (value SectionType) apply(s *Section) { s.value = value }

type applySection interface{ apply(*Section) }

//goland:noinspection GoExportedFuncWithUnexportedType
func WithSection(value SectionType) applySection { return value }
func (s Section) SectionValue() SectionType      { return s.value }
