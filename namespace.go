package namespace

import (
	"bytes"
	"errors"
	"io/ioutil"
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
	flags    int64
	prefix   string
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
	ns.prefix = targetNSPath

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
				ns.flags |= int64(nst)
			}
		}
	}

	return ns, nil
}

func Cat(filenames []string, pid int, procfs ...string) ([]*bytes.Buffer, error) {
	options := make([]Option, 0, 4)
	options = append(options, Pid(pid), Types(MNT))
	if len(procfs) == 1 {
		options = append(options, Prefix(procfs[0]))
	}

	ns, err := New(options...)
	if err != nil {
		return nil, err
	}

	fds := make([]int, 2)
	if err = syscall.Pipe(fds); err != nil {
		return nil, err
	}

	var (
		data      []byte
		delimiter = []byte{'\r', '\n', '\r', '\n', 0, 0, 0, 0}
	)

	err = ns.Execute(func(_ ...interface{}) error {
		_ = syscall.Close(fds[0])
		defer syscall.Close(fds[1])

		for index, filename := range filenames {
			if data, err = ioutil.ReadFile(filename); err != nil {
				// TODO: skip error for next file
				continue
			}

			if index != 0 {
				if _, err = syscall.Write(fds[1], delimiter); err != nil {
					return err
				}
			}

			if _, err = syscall.Write(fds[1], data); err != nil {
				return err
			}
		}

		return err
	})

	if err != nil {
		return nil, err
	}

	_ = syscall.Close(fds[1])
	defer syscall.Close(fds[0])

	buf := bytes.NewBuffer(make([]byte, 0, 2048))
	data = make([]byte, 1024)
	var n int
	for {
		if n, err = syscall.Read(fds[0], data); err != nil {
			return nil, err
		}

		if n == 0 {
			break
		}

		buf.Write(data[:n])
	}

	datas := bytes.Split(buf.Bytes(), delimiter)
	if len(datas) != len(filenames) {
		return nil, errors.New("unexpected error")
	}

	buffers := make([]*bytes.Buffer, 0, len(filenames))
	for _, data = range datas {
		buffers = append(buffers, bytes.NewBuffer(data))
	}

	return buffers, nil
}

// namespace represents a system namespace
type namespace struct {
	path string
	nst  nstype
	fd   int
}
