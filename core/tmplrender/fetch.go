package tmplrender

import (
	"context"
	"errors"

	"github.com/opensvc/om3/v3/core/naming"
)

type (
	// Fetcher reads the values no event carries: a datastore key, the
	// value of a keyword as a node evaluates it, a resource information.
	// They are fetched when a render reads them, the events only telling
	// they changed.
	Fetcher interface {
		// DataKey returns the decoded value of the key name of the
		// datastore p, ErrNotFound when the datastore or the key does
		// not exist.
		DataKey(ctx context.Context, p naming.Path, name string) ([]byte, error)

		// DataKeyNames returns the names of the keys of the datastore
		// p, ErrNotFound when it does not exist.
		DataKeyNames(ctx context.Context, p naming.Path) ([]string, error)

		// ConfigValue returns the value of the keyword kw of the object
		// p evaluated as the node evaluates it, or as the node serving
		// the api does when node is empty.
		ConfigValue(ctx context.Context, p naming.Path, kw, node string) (string, error)

		// ResourceInfo returns the value of the key of the information
		// of the resource rid of the object p on the node.
		ResourceInfo(ctx context.Context, p naming.Path, node, rid, key string) (string, error)
	}
)

// ErrNotFound is the error of a fetch of a value that does not exist.
var ErrNotFound = errors.New("not found")
