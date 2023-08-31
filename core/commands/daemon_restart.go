package commands

type (
	CmdDaemonRestart struct {
		CmdDaemonStart
	}
)

func (t *CmdDaemonRestart) Run() error {
	stop := CmdDaemonStop{OptsGlobal: t.OptsGlobal}
	if err := stop.Run(); err != nil {
		return err
	}
	if err := t.CmdDaemonStart.Run(); err != nil {
		return err
	}
	return nil
}
