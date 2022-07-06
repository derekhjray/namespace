//go:build linux
// +build linux

package namespace

/*
#define _GNU_SOURCE
#include <sched.h>
#include <linux/sched.h>
#include <stdint.h>
#include <unistd.h>
#include <string.h>
#include <stdio.h>
#include <errno.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <sys/wait.h>
#include <stdlib.h>

typedef struct {
    int fd;
    char name[8];
    char path[64];
} namespace_t;

typedef struct {
    int64_t flags;
	char nspath[32];
    namespace_t namespaces[6];
} context_t;

extern int setns(int fd, int nstype);
static int ns_init(context_t *ctx) {
    if (NULL == ctx) {
        fprintf(stderr, "nil namespace context\n");
        return -1;
    }

	int index = 0;
	int64_t flags[] = { CLONE_NEWIPC, CLONE_NEWUTS, CLONE_NEWUSER, CLONE_NEWPID, CLONE_NEWNET, CLONE_NEWNS };
	char *names[] = { "ipc", "uts", "user", "pid", "net", "mnt" };

	int size = sizeof(flags) / sizeof(int64_t);
    int i;
	for (i = 0; i < size; i++) {
		if ((ctx->flags & flags[i]) == flags[i]) {
			memcpy(ctx->namespaces[index].name, names[i], strlen(names[i]));
			sprintf(ctx->namespaces[index].path, "%s/%s", ctx->nspath, names[i]);
			ctx->namespaces[index].fd = open(ctx->namespaces[index].path, O_RDONLY | O_CLOEXEC);
			if (-1 == ctx->namespaces[index].fd) {
				fprintf(stderr, "enter %s namespace failed, reason: %s\n", ctx->namespaces[index].name, strerror(errno));
				return -1;
			}

			if (-1 == setns(ctx->namespaces[index].fd, 0)) {
				fprintf(stderr, "enter %s namespace failed, reason: %s\n", ctx->namespaces[index].name, strerror(errno));
				return -1;
			}

			index++;
		}
	}

	return 0;
}

static int ns_deinit(context_t *ctx) {
    if (NULL == ctx) {
        fprintf(stderr, "nil namespace context\n");
        return -1;
    }

	int i;
	for (i = 0; i < 6; i++) {
		if (ctx->namespaces[i].fd > 0) {
			close(ctx->namespaces[i].fd);
			ctx->namespaces[i].fd = -1;
		}
	}
}

extern int cgoNsExecute(uintptr_t);
static int ns_do(char *path, long long int flags, uintptr_t fn) {
	pid_t pid = fork();
	if (-1 == pid) {
		fprintf(stderr, "create namespace executor process failed, reason: %s\n", strerror(errno));
		return 1;
	} else if (0 == pid) {
		context_t ctx;
		memset(&ctx, 0, sizeof(context_t));
		ctx.flags = flags;
		memcpy(ctx.nspath, path, strlen(path));

		if (-1 == ns_init(&ctx)) {
			ns_deinit(&ctx);
			exit(1);
		}

		cgoNsExecute(fn);

		ns_deinit(&ctx);
		exit(0);
	}

	wait(NULL);

	return 0;
}
*/
import "C"

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/cgo"
	"syscall"
	"unsafe"
)

//export cgoNsExecute
func cgoNsExecute(fn C.uintptr_t) C.int {
	if task, ok := cgo.Handle(fn).Value().(func() int); ok {
		return C.int(task())
	}

	return 1
}

var errNSExecFailed = errors.New("namespace execute failed")

func (ns *Namespace) execute(fn func() int) error {
	cpath := C.CString(ns.prefix)
	defer C.free(unsafe.Pointer(cpath))

	if C.ns_do(cpath, C.longlong(ns.flags), C.uintptr_t(cgo.NewHandle(fn))) != 0 {
		return errNSExecFailed
	}

	return nil
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
		if ns.flags&MNT == MNT {
			fn = func(_ ...interface{}) error {
				bin, e := exec.LookPath(program)
				if e != nil {
					return e
				}

				return syscall.Exec(bin, cmdline, nil)
			}
		} else {
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

				return nil
			}
		}
	}

	if len(ns.currents) == 0 {
		return fn(args...)
	}

	if ns.flags&MNT == MNT {
		return ns.execute(func() int {
			err = fn(args...)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "excute task in specified namespaces failed, reason: %v\n", err)
				return 1
			}

			return 0
		})
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
