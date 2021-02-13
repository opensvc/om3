package selection

type (
	// Type is the selection structure
	Type struct {
		SelectorExpression string
	}
)

// New allocates a new object selection
func New(selector string) Type {
	t := Type{
		SelectorExpression: selector,
	}
	return t
}

// Status executes Status on all selected objects
func (t Type) Status() error {
	return nil
}
