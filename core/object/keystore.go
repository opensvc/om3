package object

const (
	// DataSectionName is the name of the section hosting keys in the sec, cfg and usr objects' configuration file.
	DataSectionName = "data"
)

type (
	// Keystore is the base type of sec, cfg and usr objects
	Keystore struct {
		Base
	}
)
