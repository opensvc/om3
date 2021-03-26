package client

type RelativesType bool
type Relatives struct{ value RelativesType }

func (value RelativesType) apply(s *Relatives) error { s.value = value; return nil }

type applyRelatives interface{ apply(*Relatives) error }

//goland:noinspection GoExportedFuncWithUnexportedType
func WithRelatives(value RelativesType) applyRelatives { return value }
func (s Relatives) RelativesValue() RelativesType      { return s.value }
