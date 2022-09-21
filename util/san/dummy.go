//go:build !linux

package san

func GetPaths() ([]Path, error) {
	return []Path{}, nil
}

func GetHostBusAdapters() ([]HostBusAdapter, error) {
	return []HostBusAdapter{}, nil
}
