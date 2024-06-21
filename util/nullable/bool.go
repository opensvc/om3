package nullable

import (
	"encoding/xml"
	"fmt"
)

type Bool struct {
	Valid bool
	Value bool
}

func (ni Bool) String() string {
	if ni.Valid {
		return fmt.Sprint(ni.Value)
	}
	return "N/A"
}

func (ni *Bool) UnmarshalText(b []byte) error {
	var value string
	value = string(b)
	switch value {
	case "N/A":
		ni.Valid = false
		return nil
	case "true":
		ni.Valid = true
		ni.Value = true
	case "false":
		ni.Valid = true
		ni.Value = false
	}
	return nil
}

func (ni Bool) MarshalText() ([]byte, error) {
	if ni.Valid {
		return []byte(fmt.Sprint(ni.Value)), nil
	}
	return []byte("N/A"), nil
}

// UnmarshalXML unmarshals XML into NullableBool
func (ni *Bool) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var value string
	if err := d.DecodeElement(&value, &start); err != nil {
		return err
	}

	switch value {
	case "N/A":
		ni.Valid = false
		return nil
	case "true":
		ni.Valid = true
		ni.Value = true
	case "false":
		ni.Valid = true
		ni.Value = false
	}
	return nil
}
