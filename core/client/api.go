package client

type (
	// API abstracts the requester and exposes the agent API methods
	API struct {
		Requester Requester
	}
)
