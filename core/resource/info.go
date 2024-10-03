package resource

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/naming"
)

type (
	Infos struct {
		ObjectPath naming.Path
		Resources  []Info
	}

	Info struct {
		RID  string
		Keys InfoKeys
	}

	InfoKeys []InfoKey

	InfoKey struct {
		Key   string
		Value string
	}

	infoer interface {
		Info(context.Context) (InfoKeys, error)
	}
)

func GetInfo(ctx context.Context, r Driver) (Info, error) {
	info := Info{
		RID: r.RID(),
		Keys: InfoKeys{
			{
				Key:   "driver",
				Value: fmt.Sprint(r.Manifest().DriverID),
			},
			{
				Key:   "standby",
				Value: fmt.Sprint(r.IsStandby()),
			},
			{
				Key:   "optional",
				Value: fmt.Sprint(r.IsOptional()),
			},
			{
				Key:   "disable",
				Value: fmt.Sprint(r.IsDisabled()),
			},
			{
				Key:   "monitor",
				Value: fmt.Sprint(r.IsMonitored()),
			},
			{
				Key:   "shared",
				Value: fmt.Sprint(r.IsShared()),
			},
			{
				Key:   "encap",
				Value: fmt.Sprint(r.IsEncap()),
			},
			{
				Key:   "restart",
				Value: fmt.Sprint(r.RestartCount()),
			},
			{
				Key:   "restart_delay",
				Value: fmt.Sprint(r.GetRestartDelay()),
			},
		},
	}
	i, ok := r.(infoer)
	if !ok {
		return info, nil
	}
	if keys, err := i.Info(ctx); err != nil {
		return info, err
	} else {
		info.Keys = append(info.Keys, keys...)
	}
	return info, nil
}

func (t InfoKey) String() string {
	return fmt.Sprintf("%#v", t)
}

func (t Infos) String() string {
	buff := t.ObjectPath.String() + "\n"
	for _, info := range t.Resources {
		buff += " " + info.RID + "\n"
		for _, key := range info.Keys {
			buff += "  " + key.String() + "\n"
		}
	}
	return buff
}

func NewInfos(p naming.Path) Infos {
	return Infos{
		ObjectPath: p,
		Resources:  make([]Info, 0),
	}
}
