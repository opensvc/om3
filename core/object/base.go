package object

type (
	// Base is the base struct embedded in all kinded objects.
	Base struct {
		Path Path
	}

	// ActionResult is a predictible type of actions return value, for reflect
	ActionResult struct {
		Path  Path
		Error error
		Data  interface{}
		Panic interface{}
	}
)

// NewActionResult allocate a new object action result, setting the path
// automatically.
func (o *Base) NewActionResult() *ActionResult {
	return &ActionResult{
		Path: o.Path,
	}
}

// Status returns the service status dataset
func (o *Base) Status(refresh bool) ActionResult {
	return *o.NewActionResult()
}

// List returns the stringified path as data
func (o *Base) List() ActionResult {
	result := o.NewActionResult()
	result.Data = o.Path.String()
	return *result
}
