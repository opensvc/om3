//go:build !linux

package san

func GetPaths() (Paths, error) {
	return Paths{}, nil
}

func GetInitiators() ([]Initiator, error) {
	return []Initiator{}, nil
}
