//go:build !linux

package san

func GetPaths() ([]Path, error) {
	return []Path{}, nil
}

func GetInitiators() ([]Initiator, error) {
	return []Initiator{}, nil
}
