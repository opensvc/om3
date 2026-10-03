package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	daemoncollector "github.com/opensvc/om3/v3/daemon/collector"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// recordingBus keeps the messages published, and supports nothing else.
type recordingBus struct {
	pubsub.Buser
	msgs   []pubsub.Messager
	labels [][]pubsub.Label
}

func (b *recordingBus) Pub(m pubsub.Messager, labels ...pubsub.Label) {
	b.msgs = append(b.msgs, m)
	b.labels = append(b.labels, labels)
}

func newCollectorActionRequest(body string, strategy string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set("strategy", strategy)
	ctx.Set("logger", plog.NewDefaultLogger())
	return ctx, rec
}

func TestPostInstanceCollectorAction(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })

	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}
	a := collector.Action{
		Path:   p,
		Action: "start",
		ExecID: uuid.New(),
		PID:    1234,
		Begin:  time.Now(),
	}
	require.NoError(t, daemoncollector.ActionPendingDir().Write(a))
	body := `{"exec_id":"` + a.ExecID.String() + `","phase":"begin"}`

	newAPI := func() (*DaemonAPI, *recordingBus) {
		bus := &recordingBus{}
		return &DaemonAPI{Bus: bus, localhost: "node1"}, bus
	}

	t.Run("publishes the pending begin", func(t *testing.T) {
		api, bus := newAPI()
		ctx, rec := newCollectorActionRequest(body, daemonauth.StrategyUX)
		require.NoError(t, api.PostInstanceCollectorAction(ctx, "localhost", p.Namespace, p.Kind, p.Name))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Len(t, bus.msgs, 1)
		msg := bus.msgs[0].(*msgbus.InstanceActionPending)
		assert.Equal(t, "node1", msg.Node)
		assert.Equal(t, collector.ActionPhaseBegin, msg.Phase)
		assert.Equal(t, a.ExecID, msg.Action.ExecID)
		assert.Contains(t, bus.labels[0], pubsub.Label{"node", "node1"}, "the node label forwards it to the peers")
	})

	t.Run("refuses a peer nodename", func(t *testing.T) {
		api, bus := newAPI()
		ctx, rec := newCollectorActionRequest(body, daemonauth.StrategyUX)
		require.NoError(t, api.PostInstanceCollectorAction(ctx, "node2", p.Namespace, p.Kind, p.Name))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Empty(t, bus.msgs)
	})

	t.Run("refuses a remote caller", func(t *testing.T) {
		api, bus := newAPI()
		ctx, rec := newCollectorActionRequest(body, "jwt")
		require.NoError(t, api.PostInstanceCollectorAction(ctx, "localhost", p.Namespace, p.Kind, p.Name))
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Empty(t, bus.msgs)
	})

	t.Run("refuses an invalid phase", func(t *testing.T) {
		api, bus := newAPI()
		ctx, rec := newCollectorActionRequest(`{"exec_id":"`+a.ExecID.String()+`","phase":"middle"}`, daemonauth.StrategyUX)
		require.NoError(t, api.PostInstanceCollectorAction(ctx, "localhost", p.Namespace, p.Kind, p.Name))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Empty(t, bus.msgs)
	})

	t.Run("answers not found without the pending file", func(t *testing.T) {
		api, bus := newAPI()
		ctx, rec := newCollectorActionRequest(`{"exec_id":"`+a.ExecID.String()+`","phase":"end"}`, daemonauth.StrategyUX)
		require.NoError(t, api.PostInstanceCollectorAction(ctx, "localhost", p.Namespace, p.Kind, p.Name))
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Empty(t, bus.msgs)
	})
}
