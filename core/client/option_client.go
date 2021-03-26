package client

type ClientType string
type Client struct{ value ClientType }

func (value ClientType) apply(s *Client) { s.value = value }

type applyClient interface{ apply(*Client) }

//goland:noinspection GoExportedFuncWithUnexportedType
func WithClient(value ClientType) applyClient { return value }
func (s Client) ClientValue() ClientType      { return s.value }
