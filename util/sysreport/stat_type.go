//go:build !darwin

package sysreport

type (
	Mode  = uint32
	Dev   = uint64
	Nlink = uint64
)
