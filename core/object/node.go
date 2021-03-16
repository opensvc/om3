package object

type (
	// T is the node struct.
	Node struct {
		paths BasePaths
	}
)

// New allocates a node.
func NewNode() *Node {
	t := &Node{}
	return t
}
