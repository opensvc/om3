package zfs

// GetProperty returns a dataset property value
func (t *Filesystem) GetProperty(prop string) (string, error) {
	return datasetGetProperty(t, prop)
}

// GetProperty sets a dataset property value
func (t *Filesystem) SetProperty(prop, value string) error {
	return datasetSetProperty(t, prop, value)
}
