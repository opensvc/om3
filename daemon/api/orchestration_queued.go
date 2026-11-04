package api

const (
	// OrchestrationStatusAccepted is the status of an orchestration the
	// monitor took on before the node answered.
	OrchestrationStatusAccepted = "accepted"

	// OrchestrationStatusQueued is the status of an orchestration the
	// monitor had not read yet when the node answered: it takes it on, or
	// refuses it, later.
	OrchestrationStatusQueued = "queued"
)

// Status returns the status a client says of the orchestration answered.
func (t OrchestrationQueued) Status() string {
	if t.Queued != nil && *t.Queued {
		return OrchestrationStatusQueued
	}
	return OrchestrationStatusAccepted
}
