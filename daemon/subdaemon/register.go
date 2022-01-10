package subdaemon

import (
	"errors"
)

func (t *T) Register(sub MainManager) error {
	if !t.regActionEnabled() {
		err := errors.New("can't register " + sub.Name() + " on disabled subRegister")
		t.log.Error().Err(err).Msg("Register failed")
		return err
	}
	subC := make(chan MainManager)
	done := make(chan string)
	t.regActionC <- registerAction{"register", subC, done}
	subC <- sub
	<-done
	return nil
}

func (t *T) UnRegister(sub MainManager) error {
	if !t.regActionEnabled() {
		err := errors.New("can't unregister " + sub.Name() + " on disabled subRegister")
		t.log.Error().Err(err).Msg("UnRegister failed")
		return err
	}
	subC := make(chan MainManager)
	done := make(chan string)
	t.regActionC <- registerAction{"unregister", subC, done}
	subC <- sub
	<-done
	return nil
}

func (t *T) subRegister() error {
	if t.regActionEnabled() {
		return errors.New("call subRegister() on enabled")
	}
	running := make(chan bool)
	t.regActionC = make(chan registerAction)
	go func() {
		defer t.Trace(t.Name() + "-subRegister")()
		running <- true
		t.regActionEnable.Enable()
		for {
			select {
			case m := <-t.regActionC:
				switch m.action {
				case "quit":
					t.regActionEnable.Disable()
					m.done <- "done"
					return
				case "get":
					for _, element := range t.subSvc {
						m.managerC <- element
					}
					close(m.managerC)
				case "register":
					sub := <-m.managerC
					name := sub.Name()
					t.log.Debug().Msgf("registering new sub %s", name)
					t.subSvc[name] = sub
					m.done <- "done"
				case "unregister":
					sub := <-m.managerC
					name := sub.Name()
					delete(t.subSvc, name)
					t.log.Debug().Msgf("unregistering sub %s", sub.Name())
					m.done <- "done"
				}
			}
		}
	}()
	<-running
	return nil
}

func (t *T) regActionEnabled() bool {
	return t.regActionEnable.Enabled()
}

func (t *T) subRegisterQuit() error {
	if !t.regActionEnabled() {
		err := errors.New("can't register on disabled subRegister")
		t.log.Error().Err(err).Msg("RegisterQuit failed")
		return err
	}
	done := make(chan string)
	t.regActionC <- registerAction{"quit", nil, done}
	<-done
	return nil
}

func (t *T) subs() chan MainManager {
	c := make(chan MainManager)
	if !t.regActionEnabled() {
		err := errors.New("can't get subs from disabled subRegister")
		t.log.Error().Err(err).Msg("subs failed")
		return c
	}
	m := registerAction{"get", c, nil}
	t.regActionC <- m
	return c
}
