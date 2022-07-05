package object

import "context"

// Restart stops then starts the local instance of the object
func (t *actor) Restart(ctx context.Context) error {
	if err := t.Stop(ctx); err != nil {
		return err
	}
	return t.Start(ctx)
}
