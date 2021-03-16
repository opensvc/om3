package object

type (
	// T is the node struct.
	Node struct {
	}
)

// New allocates a node.
func NewNode() *Node {
	t := &Node{}
	return t
}
