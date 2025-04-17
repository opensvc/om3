package commoncmd

func MergeSelector(selector string, subsysSelector string, kind string, defaultSelector string) string {
	var s string
	switch {
	case selector != "":
		s = selector
	case subsysSelector != "":
		s = subsysSelector
	default:
		s = defaultSelector
	}
	if (subsysSelector != "") && (kind != "") {
		kindSelector := "*/" + kind + "/*"
		if s == "" {
			s = kindSelector
		} else {
			s += "+" + kindSelector
		}
	}
	return s
}
