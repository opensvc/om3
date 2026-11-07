package daemonapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/key"
)

// configCheck runs the checks a configuration write passes before it lands,
// and keeps what they find.
//
// A write stops at the first problem, and is refused for it. A dry run of
// the write, and the validation of a draft, run every check instead, and
// answer what they found as alerts: the author of a configuration learns in
// one request everything a write of it would be refused for, with the
// warnings of the validation, which a write does not answer.
//
// The write, its dry run and the validation of a draft share the checks, so
// what a dry run says is what the write does.
type configCheck struct {
	ctx      echo.Context
	nodename string
	p        naming.Path

	// collect has every check run, and every problem kept as alerts.
	collect bool

	alerts  api.ConfigAlertItems
	refusal *configRefusal

	// denied is set when the keyword policy refused something, so the
	// checks judging only what the policy allows are not run: the holds of
	// a configuration name the objects of other namespaces, which a
	// keyword the caller may not write is not a way to ask about.
	denied bool
}

// configRefusal is a problem a write is refused for: the answer the write
// gives it, and the error a caller of the checks, as an update, tells it by.
type configRefusal struct {
	status int
	title  string
	detail string
	err    error
}

// newConfigCheck returns the checks of a write of the configuration of p,
// and with collect, the checks of its dry run, or of the validation of a
// draft.
func (a *DaemonAPI) newConfigCheck(ctx echo.Context, p naming.Path, collect bool) *configCheck {
	return &configCheck{
		ctx:      ctx,
		nodename: a.localhost,
		p:        p,
		collect:  collect,
		alerts:   make(api.ConfigAlertItems, 0),
	}
}

// refuse keeps a problem of the write, and the alerts saying it, and says
// whether the checks go on: a write stops at the first problem.
func (c *configCheck) refuse(r configRefusal, alerts ...api.ConfigAlert) bool {
	if c.refusal == nil {
		c.refusal = &r
	}
	c.alerts = append(c.alerts, alerts...)
	return c.collect
}

// refused says a check found a problem the write is refused for.
func (c *configCheck) refused() bool {
	return c.refusal != nil
}

// answer answers the request with what the checks found: every alert, for a
// dry run or a validation, or else the refusal of the write.
func (c *configCheck) answer() error {
	if c.collect {
		return c.ctx.JSON(http.StatusOK, api.ConfigAlertList{Kind: "ConfigAlertList", Items: c.alerts})
	}
	return JSONProblemf(c.ctx, c.refusal.status, c.refusal.title, "%s", c.refusal.detail)
}

// alert returns the error alert of a problem the write is refused for, in
// the words of the alerts of the validation.
func (c *configCheck) alert(kind string, k key.T, comment string) api.ConfigAlert {
	return api.ConfigAlert{
		Node:    c.nodename,
		Path:    c.p.String(),
		Level:   api.ConfigAlertLevelError,
		Kind:    kind,
		Key:     k.String(),
		Comment: comment,
	}
}

// rbac checks the keywords the write changes against the grants of the
// caller. Each refusal is an alert of kind denied, by the keyword or the
// section it is about.
func (c *configCheck) rbac(body []byte) bool {
	l := configRbacRefusals(c.ctx, c.p, body)
	if len(l) == 0 {
		return true
	}
	c.denied = true
	err := errors.Join(l...)
	alerts := make([]api.ConfigAlert, len(l))
	for i, e := range l {
		var k key.T
		var r keyRefusal
		if errors.As(e, &r) {
			k = r.key
		}
		alerts[i] = c.alert(api.ConfigAlertKindDenied, k, trimSentinel(e, ErrDenied))
	}
	return c.refuse(configRefusal{
		status: http.StatusForbidden,
		title:  "Forbidden",
		detail: fmt.Sprintf("Config validation: %s", err),
		err:    err,
	}, alerts...)
}

// stopToChange checks the write changes nothing a resource up would leak by.
func (c *configCheck) stopToChange(body []byte, force bool) bool {
	err := stopToChange(c.ctx, c.p, body, force)
	if err == nil {
		return true
	}
	kind := api.ConfigAlertKindDenied
	if errors.Is(err, object.ErrStopToChange) {
		kind = api.ConfigAlertKindResourceUp
	}
	return c.refuse(configRefusal{
		status: http.StatusForbidden,
		title:  "Forbidden",
		detail: err.Error(),
		err:    err,
	}, c.alert(kind, key.T{}, trimSentinel(err, ErrDenied)))
}

// validate keeps the alerts of the validation of the configuration, the
// warnings with the errors, and refuses the write for an error, as refusal
// says it.
func (c *configCheck) validate(alerts xconfig.Alerts, refusal func() configRefusal) bool {
	c.alerts = append(c.alerts, configAlertItems(c.nodename, alerts)...)
	if !alerts.HasError() {
		return true
	}
	return c.refuse(refusal())
}

// claims checks the write takes the namespace over none of its claims. A
// claim the dry run or the validation could not weigh is an error of the
// request rather than an alert: it says nothing of the configuration.
func (c *configCheck) claims(o any, cfg *xconfig.T) (bool, error) {
	err := refuseClaimOverrun(c.ctx.Request().Context(), c.p, o, cfg)
	switch {
	case err == nil:
		return true, nil
	case c.collect && !errors.Is(err, ErrClaimOverrun):
		return false, err
	}
	return c.refuse(configRefusal{
		status: http.StatusForbidden,
		title:  "Forbidden",
		detail: err.Error(),
		err:    err,
	}, c.alert(api.ConfigAlertKindClaimOverrun, key.T{}, trimSentinel(err, ErrClaimOverrun))), nil
}

// holds checks the resources of the configuration hold no value another
// object's resources hold, on the nodes of scope. Each conflict is an alert,
// by the section of the resource.
//
// Only a configuration the keyword policy allows is judged, as a write
// judges only those: a dry run or a validation refused by the policy says
// nothing of the holds.
func (c *configCheck) holds(body []byte, scope []string, force bool) (bool, error) {
	if c.denied {
		return true, nil
	}
	err := refuseHoldConflicts(c.ctx, c.p, body, scope, force)
	var held holdConflictError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &held):
		alerts := make([]api.ConfigAlert, len(held.conflicts))
		for i, s := range held.conflicts {
			rid := held.rids[i]
			alerts[i] = c.alert(api.ConfigAlertKindHeld, key.T{Section: rid}, strings.TrimPrefix(s, rid+": "))
		}
		return c.refuse(configRefusal{
			status: http.StatusConflict,
			title:  api.ProblemTitleHeld,
			detail: err.Error(),
			err:    err,
		}, alerts...), nil
	case errors.Is(err, ErrDenied):
		return c.refuse(configRefusal{
			status: http.StatusForbidden,
			title:  "Forbidden",
			detail: err.Error(),
			err:    err,
		}, c.alert(api.ConfigAlertKindDenied, key.T{}, trimSentinel(err, ErrDenied))), nil
	default:
		return false, err
	}
}

// trimSentinel returns the message of err without the sentinel heading it,
// which the kind of the alert says already.
func trimSentinel(err, sentinel error) string {
	return strings.TrimPrefix(err.Error(), sentinel.Error()+": ")
}
