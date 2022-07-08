package object

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/funcopt"
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
func NewNode(opts ...funcopt.O) *Node {
	t := &Node{}
	t.init(opts...)
	return t
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

func (t Node) IsVolatile() bool {
	return t.volatile
}
