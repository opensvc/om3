package hostname

import "regexp"

const regexStringRFC952 = `^[a-zA-Z]([a-zA-Z0-9\-]+[\.]?)*[a-zA-Z0-9]$` // https://tools.ietf.org/html/rfc952
var regexRFC952 = regexp.MustCompile(regexStringRFC952)

func IsValid(s string) bool {
	return regexRFC952.MatchString(s)
}
