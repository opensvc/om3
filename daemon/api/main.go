//go:generate oapi-codegen --config=codegen_server.yaml ./api.yaml
//go:generate oapi-codegen --config=codegen_type.yaml ./api.yaml
//go:generate oapi-codegen --config=codegen_client.yaml ./api.yaml

package api

import "fmt"

func (t OrchestrationQueued) String() (out string) {
	return fmt.Sprint(t.OrchestrationId)
}

func (t Problem) String() (out string) {
	if t.Status != 200 {
		out += fmt.Sprintf("%d ", t.Status)
	}
	out += fmt.Sprintf(t.Title)
	if t.Detail != "" {
		out += fmt.Sprintf(": %s", t.Detail)
	}
	return
}
