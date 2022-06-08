//go:build linux
// +build linux

package namespace

import (
	"runtime"
	"syscall"
)

// SYS_SETNS syscall allows changing the namespace of the current process.
var SYS_SETNS = map[string]uintptr{
	"386":     346,
	"amd64":   308,
	"arm64":   268,
	"arm":     375,
	"ppc64":   350,
	"ppc64le": 350,
	"s390x":   339,
}[runtime.GOARCH]

const (
	//MNT  = syscall.CLONE_NEWNS
	UTS  = syscall.CLONE_NEWUTS
	IPC  = syscall.CLONE_NEWIPC
	USER = syscall.CLONE_NEWUSER
	PID  = syscall.CLONE_NEWPID
	NET  = syscall.CLONE_NEWNET
)

func (n nstype) String() string {
	switch n {
	//case MNT:
	//	return "mnt"
	case UTS:
		return "uts"
	case IPC:
		return "ipc"
	case USER:
		return "user"
	case PID:
		return "pid"
	case NET:
		return "net"
	}

	return ""
}

// Execute executes a func in specified namespaces, args specify arguments used by fn
func (ns *Namespace) Execute(fn func(...interface{}) error, args ...interface{}) (err error) {
	if len(ns.currents) == 0 {
		return fn(args...)
	}

	resumeIndex := 0

	// lock current thread, avoid other functions executed in new namespaces accidentally
	runtime.LockOSThread()

	defer func() {
		for index := range ns.targets {
			ns.targets[index].deinit()
		}

		for index := range ns.currents {
			if index <= resumeIndex {
				ns.currents[index].enter()
			}
			ns.currents[index].deinit()
		}

		runtime.UnlockOSThread()
	}()

	for index := range ns.currents {
		if err = ns.currents[index].init(); err != nil {
			return err
		}
	}

	for index := range ns.targets {
		if err = ns.targets[index].init(); err != nil {
			return err
		}

		if err = ns.targets[index].enter(); err != nil {
			return err
		}

		resumeIndex = index
	}

	return fn(args...)
}

// enter sets the current namespace to the namespace represented
// by namespace.
func (ns *namespace) enter() (err error) {
	switch ns.nst {
	case UTS, IPC, USER, PID, NET:
	default:
		return ErrNotImplemented
	}

	_, _, e1 := syscall.RawSyscall(SYS_SETNS, uintptr(ns.fd), uintptr(ns.nst), 0)
	if e1 != 0 {
		err = e1
	}

	return
}

// init initializes namespace file descriptor if not initialized
func (ns *namespace) init() error {
	if ns.active() {
		return nil
	}

	fd, err := syscall.Open(ns.path, syscall.O_RDONLY, 0)
	if err != nil {
		return err
	}

	ns.fd = fd

	return nil
}

// active returns true if Close() has not been called.
func (ns *namespace) active() bool {
	return ns.fd != -1
}

// deinit closes the Namespace and resets its file descriptor to -1.
// It is not safe to use a Namespace after Close() is called.
func (ns *namespace) deinit() error {
	if ns.fd == -1 {
		return nil
	}

	if err := syscall.Close(ns.fd); err != nil {
		return err
	}

	ns.fd = -1

	return nil
}
