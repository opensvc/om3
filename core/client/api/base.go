package api

type Base struct {
	action string
	method string
	node   string
	client Requester
}

func (t Base) GetAction() string {
	return string(t.action)
}
func (t Base) GetMethod() string {
	return string(t.method)
}
func (t Base) GetNode() string {
	return string(t.node)
}
func (t *Base) SetAction(s string) {
	t.action = s
}
func (t *Base) SetMethod(s string) {
	t.method = s
}
func (t *Base) SetNode(s string) {
	t.node = s
}
func (t *Base) SetClient(i interface{}) {
	t.client = i.(Requester)
}
