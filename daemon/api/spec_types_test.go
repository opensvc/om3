package api

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/opensvc/om3/v3/core/instance"
)

// jsonFields returns the names a struct is encoded with, and whether each is
// always encoded: not omitted when empty or zero, and not the field of a
// struct embedded by pointer, which is not encoded at all when nil. The fields of an embedded
// struct are encoded as its own.
func jsonFields(t reflect.Type, always bool, fields map[string]bool) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "-" || !f.IsExported() {
			continue
		}
		if f.Anonymous && name == "" {
			jsonFields(f.Type, always && f.Type.Kind() != reflect.Pointer, fields)
			continue
		}
		if name == "" {
			name = f.Name
		}
		// omitempty leaves a struct, as a time.Time, encoded: omitzero
		// does not.
		l := strings.Split(opts, ",")
		omitted := slices.Contains(l, "omitzero") ||
			slices.Contains(l, "omitempty") && f.Type.Kind() != reflect.Struct
		fields[name] = always && !omitted
	}
}

// A schema the generated code reads and writes as a Go type, by
// x-go-type, documents that type: the code compiles and works whatever the
// schema says, so nothing else tells a field added to the type without the
// schema, and the spec served at /api/openapi, and the clients generated
// from it, miss it. The properties are the names the type is encoded with,
// and the required ones are encoded always.
func TestSpecSchemasDescribeTheirGoTypes(t *testing.T) {
	spec, err := GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	for name, typ := range map[string]reflect.Type{
		"InstanceConfig": reflect.TypeFor[instance.Config](),
		"ResourceConfig": reflect.TypeFor[instance.ResourceConfig](),
	} {
		t.Run(name, func(t *testing.T) {
			ref, ok := spec.Components.Schemas[name]
			if !ok || ref.Value == nil {
				t.Fatalf("no %s schema", name)
			}
			schema := ref.Value
			fields := make(map[string]bool)
			jsonFields(typ, true, fields)
			for field := range fields {
				if _, ok := schema.Properties[field]; !ok {
					t.Errorf("%s is encoded with %s, which the schema does not say", typ, field)
				}
			}
			for property := range schema.Properties {
				if _, ok := fields[property]; !ok {
					t.Errorf("the schema says %s, which %s is not encoded with", property, typ)
				}
			}
			for _, required := range schema.Required {
				if always, ok := fields[required]; ok && !always {
					t.Errorf("the schema requires %s, which %s does not always encode", required, typ)
				}
			}
			for field, always := range fields {
				if always && !slices.Contains(schema.Required, field) {
					t.Errorf("%s always encodes %s, which the schema does not require", typ, field)
				}
			}
		})
	}
}
