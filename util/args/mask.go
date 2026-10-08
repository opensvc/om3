package args

import "strings"

// secretFlags are the command flags whose values may hold secrets: a
// keyword value, or an environment variable exported to the action.
var secretFlags = []string{"--value", "--env"}

// MaskSecrets returns a copy of argv with the values of the secretFlags masked,
// as given in the "--flag value" or the "--flag=value" form.
//
// An --env value keeps the name of the variable it sets, as NAME=xxx: which
// variable an action got is worth reading, its value is not.
func MaskSecrets(argv []string) []string {
	masked := make([]string, len(argv))
	copy(masked, argv)
	for i := 0; i < len(masked); i++ {
		for _, flag := range secretFlags {
			switch {
			case masked[i] == flag:
				if i+1 < len(masked) {
					masked[i+1] = maskFlagValue(flag, masked[i+1])
					i++
				}
			case strings.HasPrefix(masked[i], flag+"="):
				masked[i] = flag + "=" + maskFlagValue(flag, strings.TrimPrefix(masked[i], flag+"="))
			default:
				continue
			}
			break
		}
	}
	return masked
}

func maskFlagValue(flag, value string) string {
	if flag == "--env" {
		if name, _, ok := strings.Cut(value, "="); ok {
			return name + "=xxx"
		}
	}
	return "xxx"
}
