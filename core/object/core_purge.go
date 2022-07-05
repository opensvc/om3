package object

import "context"

// Purge is the 'purge' object action entrypoint.
// It chains unprovision and delete actions.
func (t actor) Purge(ctx context.Context) error {
	if err := t.Unprovision(ctx); err != nil {
		return err
	}
	return t.Delete(ctx)
}
