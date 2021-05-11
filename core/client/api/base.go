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

//
// UnsetNode zeroes the node field, which usually contains a sane default.
// Set refuses to assign an empty string to node.
//
func (t *Base) UnsetNode(s string) {
	t.node = ""
}

// Set refuses to assign an empty string to node.
func (t *Base) SetNode(s string) {
	if s == "" {
		//
		// Don't overwrite the default for an empty string.
		// Explicitely use UnsetNode() if you really want to.
		//
		return
	}
	t.node = s
}
func (t *Base) SetClient(i interface{}) {
	t.client = i.(Requester)
}
