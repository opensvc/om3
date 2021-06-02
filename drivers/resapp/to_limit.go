package resapp

import "opensvc.com/opensvc/util/limits"

func (t T) toLimits() (l limits.T) {
	if t.LimitNoFile != nil {
		l.LimitNoFile = *t.LimitNoFile
	}
	if t.LimitStack != nil {
		l.LimitStack = *t.LimitStack
	}
	if t.LimitMemLock != nil {
		l.LimitMemLock = *t.LimitMemLock
	}
	if t.LimitNProc != nil {
		l.LimitNProc = *t.LimitNProc
	}
	if t.LimitVMem != nil {
		l.LimitVMem = *t.LimitVMem
	}
	if t.LimitCpu != nil {
		l.LimitCpu = *t.LimitCpu
	}
	if t.LimitCore != nil {
		l.LimitCore = *t.LimitCore
	}
	if t.LimitData != nil {
		l.LimitData = *t.LimitData
	}
	if t.LimitFSize != nil {
		l.LimitFSize = *t.LimitFSize
	}
	if t.LimitRss != nil {
		l.LimitRss = *t.LimitRss
	}
	return
}
