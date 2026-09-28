package daemonapi

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

type instanceItemResponse struct {
	api.InstanceItem
}

func newInstanceItemResponse(path naming.Path, nodename string, config *instance.Config, monitor *instance.Monitor, status *instance.Status) instanceItemResponse {
	return instanceItemResponse{InstanceItem: api.InstanceItem{
		Kind: "InstanceItem",
		Meta: api.InstanceMeta{
			Node:   nodename,
			Object: path.String(),
		},
		Data: api.Instance{
			Config:  config,
			Monitor: monitor,
			Status:  status,
		},
	}}
}

func (t instanceItemResponse) MarshalJSON() ([]byte, error) {
	// Instance data is shared with the daemon caches and other API responses.
	// Normalize the wire copy so zero dates become null without mutating those
	// shared structures or changing their internal serialization.
	b, err := json.Marshal(t.InstanceItem)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	var item map[string]any
	if err := decoder.Decode(&item); err != nil {
		return nil, err
	}
	normalizeInstanceItemDates(item)
	return json.Marshal(item)
}

func normalizeInstanceItemDates(item map[string]any) {
	data := mapField(item, "data")
	config := mapField(data, "config")
	nullZeroTimeFields(config, "updated_at")

	monitor := mapField(data, "monitor")
	nullZeroTimeFields(monitor,
		"global_expect_updated_at",
		"local_expect_updated_at",
		"state_updated_at",
		"monitor_action_executed_at",
		"updated_at",
	)
	for _, value := range mapField(monitor, "resources") {
		restart := mapField(mapValue(value), "restart")
		nullZeroTimeFields(restart, "last_at")
	}

	normalizeInstanceStatusDates(mapField(data, "status"))
}

func normalizeInstanceStatusDates(status map[string]any) {
	nullZeroTimeFields(status, "frozen_at", "last_started_at", "stopped_at", "updated_at")

	for _, value := range mapField(status, "encap") {
		normalizeInstanceStatusDates(mapValue(value))
	}
	for _, value := range mapField(status, "resources") {
		resource := mapValue(value)
		provisioned := mapField(resource, "provisioned")
		nullZeroTimeFields(provisioned, "mtime")
		for _, file := range sliceField(resource, "files") {
			nullZeroTimeFields(mapValue(file), "mtime")
		}
	}
	for _, running := range sliceField(status, "running") {
		nullZeroTimeFields(mapValue(running), "at")
	}
}

func nullZeroTimeFields(object map[string]any, fields ...string) {
	for _, field := range fields {
		value, ok := object[field].(string)
		if !ok {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err == nil && parsed.IsZero() {
			object[field] = nil
		}
	}
}

func mapField(object map[string]any, field string) map[string]any {
	return mapValue(object[field])
}

func mapValue(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func sliceField(object map[string]any, field string) []any {
	values, _ := object[field].([]any)
	return values
}
