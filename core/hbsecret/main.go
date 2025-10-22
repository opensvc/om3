package hbsecret

type Secret struct {
	// Version represents the current version of the heartbeat secret used by
	// localhost to encrypt the heartbeat messages.
	Version uint64 `json:"version"`

	// AltVersion represents the version of the alternate heartbeat secret used by
	// localhost to encrypt/decrypt the heartbeat messages during heartbeat secret rotation.
	AltVersion uint64 `json:"alt_version,omitempty"`

	// These fields are private and not exposed in the daemon’s data, JSON output, or events
	secret    string
	altSecret string
}

func NewSecret(secret, altSecret string, version, altVersion uint64) *Secret {
	return &Secret{
		Version:    version,
		AltVersion: altVersion,
		secret:     secret,
		altSecret:  altSecret,
	}
}

func (s *Secret) MainSecret() string {
	if s == nil {
		return ""
	}
	return s.secret
}

func (s *Secret) MainVersion() uint64 {
	if s == nil {
		return 0
	}
	return s.Version
}

func (s *Secret) AltSecret() string {
	if s == nil {
		return ""
	}
	return s.altSecret
}

func (s *Secret) AltSecretVersion() uint64 {
	if s == nil {
		return 0
	}
	return s.AltVersion
}

func (s *Secret) DeepCopy() *Secret {
	v := *s
	return &v
}

func (s *Secret) Rotate() {
	oldS := s.secret
	oldV := s.Version
	s.secret = s.altSecret
	s.Version = s.AltVersion
	s.altSecret = oldS
	s.AltVersion = oldV
}
