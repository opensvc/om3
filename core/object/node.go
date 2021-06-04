package object

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/logging"
	"opensvc.com/opensvc/util/xsession"
)

type (
	// Node is the node struct.
	Node struct {
		//private
		log      zerolog.Logger
		volatile bool

		// caches
		id         uuid.UUID
		configFile string
		config     *xconfig.T
		paths      NodePaths
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
		t.log.Debug().Msgf("%s init error: %s", t, err)
		return err
	}

	t.log = logging.Configure(logging.Config{
		ConsoleLoggingEnabled: true,
		EncodeLogsAsJSON:      true,
		FileLoggingEnabled:    true,
		Directory:             t.LogDir(),
		Filename:              "node.log",
		MaxSize:               5,
		MaxBackups:            1,
		MaxAge:                30,
	}).
		With().
		Str("n", hostname.Hostname()).
		Str("sid", xsession.ID).
		Logger()

	if err := t.loadConfig(); err != nil {
		t.log.Debug().Msgf("%s init error: %s", t, err)
		return err
	}
	t.log.Debug().Msgf("%s initialized", t)
	return nil
}

func (t Node) String() string {
	return fmt.Sprintf("node")
}

func (t Node) IsVolatile() bool {
	return t.volatile
}
