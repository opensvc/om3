package commoncmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// DefaultWait is how long one request is held when no duration is asked for.
// It is the longest the daemon holds one, and a wait with no duration asks
// again for as long as it takes, so the default is "until it ends".
const DefaultWait = time.Hour

// ErrStillRunning says a wait expired on work that has not ended. It is not
// a failure of the work, nor of the request, which is why a wait with no
// duration asks again rather than reporting it.
var ErrStillRunning = errors.New("still running when the wait expired")

// SetWait reads what the caller asked to wait for.
//
// No duration is a wait with no end: the daemon holds one request for an hour
// at most, so the wait is that hour asked again for as long as it takes,
// which is what unbounded says. A duration that is not positive is refused,
// rather than silently making a wait command a listing that reports success
// on work still running.
func SetWait(cmd *cobra.Command, wait *time.Duration, unbounded *bool) error {
	if !cmd.Flags().Changed("duration") {
		*wait = DefaultWait
		*unbounded = true
		return nil
	}
	if *wait <= 0 {
		return fmt.Errorf("--duration must be positive, got %s", *wait)
	}
	return nil
}

// IsStillRunning says an answer is nothing but waits that expired on work
// still running, which is the one answer worth asking again for. A node that
// failed to answer at all is not.
func IsStillRunning(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range joined.Unwrap() {
			if !IsStillRunning(e) {
				return false
			}
		}
		return true
	}
	return errors.Is(err, ErrStillRunning)
}

// NewCmdDaemonExecWait returns the command waiting for the end of an exec.
func NewCmdDaemonExecWait(defaultNodeSelector string) *cobra.Command {
	options := CmdDaemonExecList{}
	cmd := &cobra.Command{
		Use:   "wait EXEC_ID",
		Short: "wait for the end of one run of one object on one node",
		Long: `The daemon holds the request until the exec ends, and answers how it went, so
this neither polls nor keeps an event stream open.

An exec that has already ended is answered at once: the daemon remembers it
for a while, which is what lets a client that lost its connection, or that
asks late, still be told how the work went.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.ExecID = args[0]
			if err := SetWait(cmd, &options.Wait, &options.Unbounded); err != nil {
				return err
			}
			return options.RunWait()
		},
	}
	CmdWithArg(cmd, `EXEC_ID  The exec id the submitter of the action was handed.`)
	flags := cmd.Flags()
	FlagNodeSelectorWithDefault(flags, &options.NodeSelector, defaultNodeSelector)
	FlagOutput(flags, &options.Output)
	FlagColor(flags, &options.Color)
	flags.DurationVar(&options.Wait, "duration", 0, "give up waiting after this duration")
	return cmd
}

// NewCmdDaemonSessionWait returns the command waiting for the end of a
// session.
func NewCmdDaemonSessionWait() *cobra.Command {
	options := CmdDaemonSessionList{}
	cmd := &cobra.Command{
		Use:   "wait SESSION_ID",
		Short: "wait for the end of a submitted command",
		Long: `A session is what was submitted, and an exec is one run of one object on one
node, so a session spans the nodes the command reached. Every node is asked,
each holds the request until its own execs have ended, and what is reported is
the fold of what they answer.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.SessionID = args[0]
			if err := SetWait(cmd, &options.Wait, &options.Unbounded); err != nil {
				return err
			}
			return options.RunWait()
		},
	}
	CmdWithArg(cmd, `SESSION_ID  The session id the submitter of the action was handed.`)
	flags := cmd.Flags()
	FlagNodeSelectorWithDefault(flags, &options.NodeSelector, "*")
	FlagOutput(flags, &options.Output)
	FlagColor(flags, &options.Color)
	flags.DurationVar(&options.Wait, "duration", 0, "give up waiting after this duration")
	return cmd
}
