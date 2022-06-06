package namespace

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

var (
	ErrNotImplemented = errors.New("not implemented")
)

type nstype int

type Namespace struct {
	proc     string
	pid      int
	types    []nstype
	currents []*namespace
	targets  []*namespace
}

// New creates a new Namespace instance
func New(options ...Option) (*Namespace, error) {
	ns := &Namespace{proc: "/proc", pid: -1}
	for _, option := range options {
		option(ns)
	}

	if ns.pid <= 0 {
		return nil, errors.New("namespace id is not specified")
	}

	var (
		cns string
		tns string
		err error
	)

	currentNSPath := filepath.Join("/proc", strconv.Itoa(os.Getpid()), "task", strconv.Itoa(syscall.Gettid()), "ns")
	targetNSPath := filepath.Join(ns.proc, strconv.Itoa(ns.pid), "ns")

	if len(ns.types) > 0 {
		ns.currents = make([]*namespace, 0, len(ns.types))
		ns.targets = make([]*namespace, 0, len(ns.types))
		for _, nst := range ns.types {
			cpath := filepath.Join(currentNSPath, nst.String())
			if cns, err = os.Readlink(cpath); err != nil {
				return nil, err
			}

			tpath := filepath.Join(targetNSPath, nst.String())
			if tns, err = os.Readlink(tpath); err != nil {
				return nil, err
			}

			// skip namespace same with current task
			if cns != tns {
				ns.currents = append(ns.currents, &namespace{path: cpath, nst: nst, fd: -1})
				ns.targets = append(ns.targets, &namespace{path: tpath, nst: nst, fd: -1})
			}
		}
	}

	return ns, nil
}

// namespace represents a system namespace
type namespace struct {
	path string
	nst  nstype
	fd   int
}
