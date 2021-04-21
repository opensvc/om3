package manifest

import (
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/util/converters"
)

type (
	//
	// T describes a driver so callers can format the input as the
	// driver expects.
	//
	// A typical allocation is:
	// m := New("fs", "flag").AddKeyword(kws...).AddContext(ctx...)
	//
	T struct {
		Group    drivergroup.T      `json:"group"`
		Name     string             `json:"name"`
		Keywords []keywords.Keyword `json:"keywords"`
		Context  []Context          `json:"context"`
	}

	//
	// Context is a key-value the resource expects to find in the input,
	// merged with keywords coming from configuration file.
	//
	// For example, a driver often needs the parent object Path, which
	// can be asked via:
	//
	// T{
	//     Context: []Context{
	//         {
	//             Key: "path",
	//             Ref:"object.path",
	//         },
	//     },
	// }
	//
	Context struct {
		// Key is the name of the key in the json representation of the context.
		Key string

		// Attr is the name of the field in the resource struct.
		Attr string

		// Ref is the code describing what context information to embed in the resource struct.
		Ref string
	}
)

var genericKeywords = []keywords.Keyword{
	{
		Option:    "disable",
		Attr:      "Disable",
		Scopable:  false,
		Required:  false,
		Converter: converters.Bool,
		Text:      "",
		Default:   "false",
	},
	{
		Option:    "optional",
		Attr:      "Optional",
		Scopable:  true,
		Required:  false,
		Converter: converters.Bool,
		Text:      "",
		Default:   "false",
	},
	{
		Option:    "tags",
		Attr:      "Tags",
		Scopable:  true,
		Required:  false,
		Converter: converters.Set,
		Text:      "A list of tags. Arbitrary tags can be used to limit action scope to resources with a specific tag. Some tags can influence the driver behaviour. For example :c-tag:`noaction` avoids any state changing action from the driver and implies ``optional=true``, :c-tag:`nostatus` forces the status to n/a.",
	},
	{
		Option:   "subset",
		Attr:     "Subset",
		Scopable: true,
		Required: false,
		Text:     "Assign the resource to a specific subset.",
	},
}

func New(group drivergroup.T, name string) *T {
	t := &T{
		Group: group,
		Name:  name,
	}
	t.Keywords = append(t.Keywords, genericKeywords...)
	return t
}

func (t *T) AddKeyword(kws ...keywords.Keyword) *T {
	t.Keywords = append(t.Keywords, kws...)
	return t
}

func (t *T) AddContext(ctx ...Context) *T {
	t.Context = append(t.Context, ctx...)
	return t
}
