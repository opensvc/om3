package fqdn

import "regexp"

const regexStringRFC1123 = `^([a-zA-Z0-9]{1}[a-zA-Z0-9_-]{0,62})(\.[a-zA-Z0-9_]{1}[a-zA-Z0-9_-]{0,62})*?(\.[a-zA-Z]{1}[a-zA-Z0-9]{0,62})\.?$`

var regexRFC1123 = regexp.MustCompile(regexStringRFC1123)

func IsValid(s string) bool {
	return regexRFC1123.MatchString(s)
}
