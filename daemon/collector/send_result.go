package collector

import (
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	// sendResult is the outcome of a send to the collector, run off the
	// loop, which applies it: the loop owns the speaker queues and caches,
	// and a sender only reads the job the loop handed it.
	sendResult interface {
		apply(t *T)
	}

	// actionSendResults is the outcome of a batch of action phases sent.
	actionSendResults []actionSendResult

	// sendOutcome is the outcome of the send of an item of a feed.
	sendOutcome int

	// objectConfigJob is an instance config the loop hands to a sender.
	objectConfigJob struct {
		path naming.Path

		// queued is the objectConfigToSend entry of path, nil when the
		// collector said it lacks the config: an entry replaced in flight
		// is not dequeued.
		queued *msgbus.InstanceConfigUpdated

		// msg is the instance config to send: queued, or the one of the
		// node holding it when queued is nil.
		msg *msgbus.InstanceConfigUpdated

		// sentChecksum is the checksum of the config the collector has.
		sentChecksum string
	}

	objectConfigItem struct {
		objectConfigJob
		outcome  sendOutcome
		checksum string
		sentAt   time.Time
		err      error
	}

	// objectConfigResult is the outcome of a batch of instance configs
	// sent: the items tried, up to the first failing.
	objectConfigResult struct {
		epoch uint64
		items []objectConfigItem
	}

	// resInfoJob is the resource info of an instance the loop hands to a
	// sender.
	resInfoJob struct {
		key string

		// queued is the resInfoToSend entry of key: an entry replaced in
		// flight is not dequeued.
		queued *msgbus.InstanceResourceInfoUpdated
	}

	resInfoItem struct {
		resInfoJob
		outcome sendOutcome
		sentAt  time.Time
		err     error
	}

	// resInfoResult is the outcome of a batch of resource infos sent: the
	// items tried, up to the first failing.
	resInfoResult struct {
		epoch uint64
		items []resInfoItem
	}
)

const (
	// sendDone is an item the collector accepted.
	sendDone sendOutcome = iota

	// sendSkipped is an item the collector has already.
	sendSkipped

	// sendDropped is an item not to send again: refused for good by the
	// collector, or which can not be read.
	sendDropped

	// sendFailed is an item to send again, as with the collector down. It
	// stops the batch.
	sendFailed
)

// sendBatchMax is the maximum number of items of a feed sent per batch, so
// the loop applies the progress of a long queue as it goes.
const sendBatchMax = 100

// sendResultChanSize is the size of sendResultC: at most one send of each
// feed is in flight, so a sender never waits for the loop to read its result.
const sendResultChanSize = 4

func (r actionSendResults) apply(t *T) {
	t.onActionSendResults(r)
}

// goSend runs send off the loop, and hands its result to the loop through
// sendResultC. Stop waits for it: the daemon cancel ends its requests.
func (t *T) goSend(send func() sendResult) {
	ctx, resultC := t.ctx, t.sendResultC
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		r := send()
		select {
		case resultC <- r:
		case <-ctx.Done():
		}
	}()
}

// newFeedEpoch starts a new feed epoch: the results of the sends cut before
// it change nothing of the queues and caches, rebuilt or left since. It is
// called on a resync with the collector, on a speaker change and on a
// collector configuration change.
func (t *T) newFeedEpoch() {
	t.feedEpoch++
}
