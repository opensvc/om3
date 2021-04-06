package object

type (
	// Node is the node struct.
	Node struct {
		paths NodePaths
	}
)

// NewNode allocates a node.
func NewNode() *Node {
	t := &Node{}
	return t
}
