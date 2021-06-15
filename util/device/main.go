package device

type (
	T string
)

func (t T) String() string {
	return string(t)
}

func (t T) RemoveHolders() error {
	for _, dev := range t.Holders() {
		if err := dev.RemoveHolders(); err != nil {
			return err
		}
		if err := dev.Remove(); err != nil {
			return err
		}
	}
	return nil
}

func (t T) Holders() []T {
	l := make([]T, 0)
	return l
}

func (t T) Remove() error {
	return nil
}
