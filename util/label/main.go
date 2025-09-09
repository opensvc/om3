package label

type (
	// M holds the key/value pairs
	M map[string]string
)

func (t M) DeepCopy() M {
	labels := make(M)
	for k, v := range t {
		labels[k] = v
	}
	return labels
}
