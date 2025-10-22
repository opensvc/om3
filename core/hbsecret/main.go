package hbsecret

type Secret struct {
	// CurrentVersion represents the current version of the heartbeat secret used by
	// localhost to encrypt the heartbeat messages.
	CurrentVersion uint64 `json:"current_version"`

	// NextVersion represents the version of the next heartbeat secret used by
	// localhost to encrypt the heartbeat messages after heartbeat secret rotation.
	NextVersion uint64 `json:"next_version,omitempty"`

	// These fields are private and not exposed in the daemon’s data, JSON output, or events
	currentSecret string
	nextSecret    string
}

func NewSecret(key, altKeay string, version, altVersion uint64) *Secret {
	return &Secret{
		CurrentVersion: version,
		NextVersion:    altVersion,
		currentSecret:  key,
		nextSecret:     altKeay,
	}
}

func (s *Secret) CurrentKey() string {
	if s == nil {
		return ""
	}
	return s.currentSecret
}

func (s *Secret) CurrentKeyVersion() uint64 {
	if s == nil {
		return 0
	}
	return s.CurrentVersion
}

func (s *Secret) NextKey() string {
	if s == nil {
		return ""
	}
	return s.nextSecret
}

func (s *Secret) NextKeyVersion() uint64 {
	if s == nil {
		return 0
	}
	return s.NextVersion
}

func (s *Secret) DeepCopy() *Secret {
	v := *s
	return &v
}

func (s *Secret) Rotate() {
	oldS := s.currentSecret
	oldV := s.CurrentVersion
	s.currentSecret = s.nextSecret
	s.CurrentVersion = s.NextVersion
	s.nextSecret = oldS
	s.NextVersion = oldV
}
