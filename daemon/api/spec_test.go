package api

import (
	"testing"
)

// The spec the daemon serves at /api/openapi is a valid OpenAPI document, so
// the clients and the tools generating code from it read it as written:
// a schema in a form the loader does not know is silently left untyped.
func TestSpecValidates(t *testing.T) {
	spec, err := GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(t.Context()); err != nil {
		t.Fatal(err)
	}
}
