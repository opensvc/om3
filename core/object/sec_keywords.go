package object

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/key"
)

var secKeywordStore = keywords.Store{
	{
		Section:  "DEFAULT",
		Option:   "cn",
		Scopable: true,
		Text:     "Certificate Signing Request Common Name.",
		Example:  "test.opensvc.com",
	},
	{
		Section:  "DEFAULT",
		Option:   "c",
		Scopable: true,
		Text:     "Certificate Signing Request Country.",
		Example:  "FR",
	},
	{
		Section:  "DEFAULT",
		Option:   "st",
		Scopable: true,
		Text:     "Certificate Signing Request State.",
		Example:  "Oise",
	},
	{
		Section:  "DEFAULT",
		Option:   "l",
		Scopable: true,
		Text:     "Certificate Signing Request Location.",
		Example:  "Gouvieux",
	},
	{
		Section:  "DEFAULT",
		Option:   "o",
		Scopable: true,
		Text:     "Certificate Signing Request Organization.",
		Example:  "OpenSVC",
	},
	{
		Section:  "DEFAULT",
		Option:   "ou",
		Scopable: true,
		Text:     "Certificate Signing Request Organizational Unit.",
		Example:  "Lab",
	},
	{
		Section:  "DEFAULT",
		Option:   "email",
		Scopable: true,
		Text:     "Certificate Signing Request Email.",
		Example:  "test@opensvc.com",
	},
	{
		Section:   "DEFAULT",
		Option:    "alt_names",
		Converter: converters.List,
		Scopable:  true,
		Text:      "Certificate Signing Request Alternative Domain Names.",
		Example:   "www.opensvc.com opensvc.com",
	},
	{
		Section:   "DEFAULT",
		Option:    "bits",
		Converter: converters.Size,
		Scopable:  true,
		Text:      "Certificate Private Key Length.",
		Default:   "4k",
		Example:   "8192",
	},
	{
		Section:   "DEFAULT",
		Option:    "validity",
		Converter: converters.Duration,
		Scopable:  true,
		Text:      "Certificate Validity duration.",
		Default:   "1y",
		Example:   "10y",
	},
	{
		Section:  "DEFAULT",
		Option:   "ca",
		Scopable: true,
		Text:     "The name of secret containing a certificate to use as a Certificate Authority. This secret must be in the same namespace.",
		Example:  "ca",
	},
}

func (t Sec) KeywordLookup(k key.T) keywords.Keyword {
	switch k.Section {
	case "data", "env":
		return keywords.Keyword{
			Option:   "*", // trick IsZero()
			Scopable: true,
			Required: false,
		}
	}
	kw := secKeywordStore.Lookup(k)
	if !kw.IsZero() {
		return kw
	}
	return keywordStore.Lookup(k)
}
