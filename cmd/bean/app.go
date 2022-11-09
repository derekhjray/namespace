package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var app App

func init() {
	if strings.HasPrefix(os.Args[0], "bean-exec") {
		var subApp App
		subApp.parseCommand(os.Args[1:]...)
		switch subApp.command {
		case "read":
			if len(subApp.artifacts) != 1 {
				Errorf("Require one artifact for read command, but got %d", len(subApp.artifacts))
				os.Exit(1)
			}

			read(subApp.artifacts[0])
		case "stat":
			if len(subApp.artifacts) != 1 {
				Errorf("Require one artifact for stat command, but got %d", len(subApp.artifacts))
				os.Exit(1)
			}
			stat(subApp.artifacts[0])
		default:
			Fatalf("Unknown bean sub-command '%s'", subApp.command)
		}

		os.Exit(0)
	}
}

type App struct {
	command   string
	artifacts []string
	namespace string
	help      bool
}

func (app *App) Execute() {
	app.parse()
	if app.help {
		app.usage()
		return
	}

	args := append([]string{"bean-exec", app.command}, app.artifacts...)
	parent, child, err := app.newPipe()
	if err != nil {
		Fatalf("Create pipe failed, %v", err)
	}

	namespaces := []string{app.namespace}

	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3"},
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}

	if err = cmd.Start(); err != nil {
		Fatalf("Start bean exec process failed, %v", err)
	}
	_ = child.Close()
	defer func() {
		_ = parent.Close()
	}()

	r := nl.NewNetlinkRequest(int(libcontainer.InitMsg), 0)
	r.AddData(&libcontainer.Bytemsg{
		Type:  libcontainer.NsPathsAttr,
		Value: []byte(strings.Join(namespaces, ",")),
	})
	if _, err = io.Copy(parent, bytes.NewReader(r.Serialize())); err != nil {
		Fatalf("Write bean namespace configure failed, %v", err)
	}

	if err = app.initWaiter(parent); err != nil {
		Fatalf("Initialize bean children process waiter failed, %v", err)
	}

	if err = cmd.Wait(); err != nil {
		Fatalf("Wait bean children process to exit failed, %v", err)
	}

	if err = app.reapChildren(parent); err != nil {
		Fatalf("Cleanup bean children process failed, %v", err)
	}
}

func (app *App) newPipe() (parent *os.File, child *os.File, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}

	parent = os.NewFile(uintptr(fds[1]), "parent")
	child = os.NewFile(uintptr(fds[0]), "child")
	return
}

// initWaiter reads back the initial \0 from runc init
func (app *App) initWaiter(r io.Reader) error {
	inited := make([]byte, 1)
	n, err := r.Read(inited)
	if err == nil {
		if n < 1 {
			err = errors.New("short read")
		} else if inited[0] != 0 {
			err = fmt.Errorf("unexpected %d != 0", inited[0])
		} else {
			return nil
		}
	}

	return err
}

func (app *App) reapChildren(parent *os.File) error {
	decoder := json.NewDecoder(parent)
	decoder.DisallowUnknownFields()
	var pid struct {
		Pid2 int `json:"stage2_pid"`
		Pid1 int `json:"stage1_pid"`
	}

	if err := decoder.Decode(&pid); err != nil {
		return err
	}

	// Reap children.
	_, _ = unix.Wait4(pid.Pid1, nil, 0, nil)
	_, _ = unix.Wait4(pid.Pid2, nil, 0, nil)

	// Sanity check.
	if pid.Pid1 == 0 || pid.Pid2 == 0 {
		return fmt.Errorf("invalid pids")
	}

	return nil
}

func (app *App) getLogs(logread *os.File) error {
	logsDecoder := json.NewDecoder(logread)
	logsDecoder.DisallowUnknownFields()
	var logentry struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	for {
		if err := logsDecoder.Decode(&logentry); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return err
		}

		if logentry.Level == "" || logentry.Msg == "" {
			return fmt.Errorf("init log: empty log entry: %+v", logentry)
		}
	}
}

func (app *App) parse() {
	args := make([]string, 0, len(os.Args))
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-h", "--help":
			app.help = true
			break
		case "-p", "--pid":
			if i+1 >= len(os.Args) {
				_, _ = fmt.Fprintf(os.Stderr, "Option '%s' requires a value\n", os.Args[i])
				app.usage()
				os.Exit(1)
			}

			pid, err := strconv.Atoi(os.Args[i+1])
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Invalid pid value '%s', %v\n", os.Args[i+1], err)
				os.Exit(1)
			}

			app.namespace = fmt.Sprintf("mnt:%s/proc/%d/ns/mnt", os.Getenv("HOSTFS"), pid)
			i++
		default:
			args = append(args, os.Args[i])
		}
	}

	if len(os.Args) == 1 {
		app.help = true
	}

	if app.help {
		return
	}

	if app.namespace == "" {
		if os.Getenv("CONTAINER_MOUNT") == "" {
			_, _ = fmt.Fprintf(os.Stderr, "Container namespace is not specified\n")
			os.Exit(1)
		}

		app.namespace = os.Getenv("CONTAINER_MOUNT")
	}

	app.parseCommand(args...)
}

func (app *App) parseCommand(args ...string) {
	app.command = args[0]
	switch args[0] {
	case "read", "stat":
		app.artifacts = args[1:]
		if len(app.artifacts) == 0 {
			_, _ = fmt.Fprintf(os.Stderr, "No artifact specified for command '%s'\n", app.command)
			app.usage()
			os.Exit(1)
		}
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Invalid beam command '%s'\n", app.command)
		app.usage()
		os.Exit(1)
	}
}

func (app *App) usage() {
	_, _ = fmt.Fprintf(os.Stderr, "Simple container file access utility\n\n")
	_, _ = fmt.Fprintf(os.Stderr, "Usage: bean [options] command artifact...\n\n")
	_, _ = fmt.Fprintf(os.Stderr, "Commands:\n")
	_, _ = fmt.Fprintf(os.Stderr, "    read    read file from specified namespace\n")
	_, _ = fmt.Fprintf(os.Stderr, "    stat    retrieve file stat from specified namespace\n")
	_, _ = fmt.Fprintf(os.Stderr, "\nOptions:\n")
	_, _ = fmt.Fprintf(os.Stderr, "    -h, --help    show bean help info\n")
	_, _ = fmt.Fprintf(os.Stderr, "    -p, --pid     specify container initial process id\n")
}
