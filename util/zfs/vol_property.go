package zfs

// GetProperty returns a dataset property value
func (t *Vol) GetProperty(prop string) (string, error) {
	return datasetGetProperty(t, prop)
}

// SetProperty sets a dataset property value
func (t *Vol) SetProperty(prop, value string) error {
	return datasetSetProperty(t, prop, value)
}
