package node

type (
	// Labels holds the key/value pairs defined in the labels section of the node.conf
	Labels map[string]string
)

func (t Labels) DeepCopy() Labels {
	labels := make(Labels)
	for k, v := range t {
		labels[k] = v
	}
	return labels
}
