package nullable

import (
	"encoding/xml"
	"fmt"
	"strconv"
)

type Int struct {
	Valid bool
	Value int
}

func (ni Int) String() string {
	if ni.Valid {
		return strconv.Itoa(ni.Value)
	}
	return "N/A"
}

func (ni *Int) UnmarshalText(b []byte) error {
	var value string
	value = string(b)
	if value == "N/A" {
		ni.Valid = false
		return nil
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	ni.Valid = true
	ni.Value = intValue
	return nil
}

func (ni Int) MarshalText() ([]byte, error) {
	if ni.Valid {
		return []byte(fmt.Sprint(ni.Value)), nil
	}
	return []byte("N/A"), nil
}

// UnmarshalXML unmarshals XML into NullableInt
func (ni *Int) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var value string
	if err := d.DecodeElement(&value, &start); err != nil {
		return err
	}

	if value == "N/A" {
		ni.Valid = false
		return nil
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	ni.Valid = true
	ni.Value = intValue
	return nil
}
