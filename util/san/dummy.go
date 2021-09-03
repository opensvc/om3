// +build !linux

package san

func Paths() ([]Path, error) {
	return []Path{}, nil
}

func HostBusAdapters() ([]HostBusAdapter, error) {
	return []HostBusAdapter{}, nil
}
