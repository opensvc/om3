package object

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/funcopt"
)

type (
	// Node is the node struct.
	Node struct {
		//private
		log      zerolog.Logger
		volatile bool

		// caches
		id           uuid.UUID
		configFile   string
		config       *xconfig.T
		mergedConfig *xconfig.T
		paths        nodePaths
	}
)

// NewNode allocates a node.
func NewNode(opts ...funcopt.O) (*Node, error) {
	t := &Node{}
	if err := t.init(opts...); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Node) init(opts ...funcopt.O) error {
	if err := funcopt.Apply(t, opts...); err != nil {
		return err
	}

	// log.Logger is configured in cmd/root.go
	t.log = log.Logger

	if err := t.loadConfig(); err != nil {
		return err
	}
	return nil
}

func (t Node) String() string {
	return fmt.Sprintf("node")
}

func (t *Node) SetVolatile(v bool) {
	t.volatile = v
}

func (t Node) IsVolatile() bool {
	return t.volatile
}
