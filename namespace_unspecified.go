//go:build !linux
// +build !linux

package namespace

import (
	"bytes"
	"github.com/derekhjray/namespace/types"
)

const (
	MNT nstype = iota
	UTS
	IPC
	USER
	PID
	NET
)

func (n nstype) String() string {
	return ""
}

func (ns *Namespace) Execute(fn func(...interface{}) error, args ...interface{}) (err error) {
	return ErrNotImplemented
}

func (ns *namespace) enter() error {
	return ErrNotImplemented
}

func (ns *namespace) init() error {
	return ErrNotImplemented
}

func (ns *namespace) active() error {
	return ErrNotImplemented
}

func (ns *namespace) deinit() error {
	return ErrNotImplemented
}

func ReadFile(filename, ns string) (*bytes.Buffer, error) {
	return nil, ErrNotImplemented
}

func Stat(filename, ns string) (*types.FileInfo, error) {
	return nil, ErrNotImplemented
}
