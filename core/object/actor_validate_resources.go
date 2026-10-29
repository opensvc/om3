package object

import (
	"github.com/opensvc/om3/v3/core/xconfig"
)

// ValidateResources returns the alerts of the resources refusing their
// configuration when the object loads them, which the validation of the
// keywords does not see: a check a driver makes in Configure, on this node.
//
// It is apart from ValidateConfig, which a write of the configuration runs
// too, and refuses on an error: a check made on the node the write lands on,
// as a cni configuration only some nodes have, may fail there and not on the
// nodes of the object.
func (t *actor) ValidateResources() xconfig.Alerts {
	alerts := make(xconfig.Alerts, 0)
	for _, r := range t.Resources() {
		if err := r.GetConfigurationError(); err != nil {
			alerts = append(alerts, t.config.NewAlertConfigure(r.RID(), r.DriverID(), err, r.IsDisabled()))
		}
	}
	return alerts
}
