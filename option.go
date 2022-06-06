package namespace

import "path/filepath"

type Option func(*Namespace)

// Types specify namespace types while creating Namespace instance, supports NET,
// UTS, IPC, USER, PID and NET.
func Types(types ...int) Option {
	return func(ns *Namespace) {
		ns.types = make([]nstype, len(types))
		for index, tp := range types {
			ns.types[index] = nstype(tp)
		}
	}
}

// Pid specify which namespaces of process to use
func Pid(pid int) Option {
	return func(ns *Namespace) {
		ns.pid = pid
	}
}

// Prefix specify prefix of directory /proc, which is useful while using this
// package in container that has bind host /proc into container
func Prefix(prefix string) Option {
	return func(ns *Namespace) {
		ns.proc = filepath.Join(prefix, "/proc")
	}
}
