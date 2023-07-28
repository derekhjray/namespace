package namespace

/*
#include "namespace.h"
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
typedef struct stat stat_t;
*/
import "C"

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/derekhjray/namespace/types"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

func ReadFile(filename, ns string) (*bytes.Buffer, error) {
	var (
		errno int
		err   error
	)

	cfile := C.CString(filename)
	cns := C.CString(ns)
	defer func() {
		C.free(unsafe.Pointer(cfile))
		C.free(unsafe.Pointer(cns))
	}()

	data := C.ns_read(cfile, cns, (*C.int)(unsafe.Pointer(&errno)))
	if errno != 0 {
		err = syscall.Errno(errno)
	}

	if data == nil {
		return nil, err
	}
	defer C.free(unsafe.Pointer(data))

	buf := C.GoBytes(unsafe.Pointer(data), C.int(C.strlen(data)))

	return bytes.NewBuffer(buf), err
}

func Stat(filename, ns string) (*types.FileInfo, error) {
	var (
		errno C.int
		err   error
	)

	cfile := C.CString(filename)
	cns := C.CString(ns)
	defer func() {
		C.free(unsafe.Pointer(cfile))
		C.free(unsafe.Pointer(cns))
	}()

	var st C.stat_t
	errno = C.ns_stat(cfile, cns, (unsafe.Pointer)(&st))
	if errno != 0 {
		err = syscall.Errno(errno)
	}

	if err != nil {
		return nil, err
	}

	fi := &types.FileInfo{
		Name:       filename,
		Uid:        int(st.st_uid),
		Gid:        int(st.st_gid),
		Size:       int64(st.st_size),
		Mode:       int64(st.st_mode),
		Inode:      int64(st.st_ino),
		BlockSize:  int64(st.st_blksize),
		Blocks:     int64(st.st_blocks),
		Links:      int64(st.st_nlink),
		AccessTime: int64(st.st_atim.tv_sec)*1e9 + int64(st.st_atim.tv_nsec),
		ModifyTime: int64(st.st_mtim.tv_sec)*1e9 + int64(st.st_mtim.tv_nsec),
	}

	fi.Perm = os.FileMode(fi.Mode).Perm().String()

	return fi, nil
}

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
	MNT  = syscall.CLONE_NEWNS
	UTS  = syscall.CLONE_NEWUTS
	IPC  = syscall.CLONE_NEWIPC
	USER = syscall.CLONE_NEWUSER
	PID  = syscall.CLONE_NEWPID
	NET  = syscall.CLONE_NEWNET
)

func (n nstype) String() string {
	switch n {
	case MNT:
		return "mnt"
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
func (ns *Namespace) Execute(command interface{}, args ...interface{}) (err error) {
	var (
		fn      func(...interface{}) error
		program string
		cmdline []string
	)
	switch v := command.(type) {
	case func(...interface{}) error:
		fn = v
	case string:
		program = v
		cmdline = make([]string, 0, len(args)+1)
		cmdline = append(cmdline, filepath.Base(program))
		for _, arg := range args {
			cmdline = append(cmdline, fmt.Sprintf("%v", arg))
		}

		args = nil
		fn = func(_ ...interface{}) error {
			var (
				stdout bytes.Buffer
				stderr bytes.Buffer
			)

			cmd := exec.Command(program, cmdline[1:]...)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err = cmd.Run(); err != nil {
				if stderr.Len() > 0 {
					err = fmt.Errorf("%v, %s", err, stderr.String())
				}

				return err
			}

			fmt.Println(stdout.String())
			return nil
		}
	}

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
