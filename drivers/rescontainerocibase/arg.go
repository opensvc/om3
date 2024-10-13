package rescontainerocibase

type (
	Arg struct {
		Option    string
		Value     string
		Obfuscate bool
		Multi     bool
		HasValue  bool
	}

	Args []Arg
)

func (a *Args) AsStrings() []string {
	var l []string
	for _, v := range *a {
		if v.HasValue {
			l = append(l, v.Option, v.Value)
		} else {
			l = append(l, v.Option)
		}
	}
	return l
}

func (a *Args) Obfuscate() []string {
	var l []string
	for _, v := range *a {
		if v.HasValue {
			if v.Obfuscate {
				l = append(l, v.Option, "obfuscate")
			} else {
				l = append(l, v.Option, v.Value)
			}
		} else {
			l = append(l, v.Option)
		}
	}
	return l
}
