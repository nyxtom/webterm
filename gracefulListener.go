package main

import (
	"net"
	"os"
	"sync"
	"syscall"
)

var httpWg sync.WaitGroup

type gracefulListener struct {
	net.Listener
	stop    chan error
	stopped bool
}

func newGracefulListener(l net.Listener) (gl *gracefulListener) {
	gl = &gracefulListener{Listener: l, stop: make(chan error)}
	go func() {
		_ = <-gl.stop
		gl.stopped = true
		gl.stop <- gl.Listener.Close()
	}()
	return
}

// Accept connections by wrapping them in a graceful connection routine and increment the wait group
func (gl *gracefulListener) Accept() (c net.Conn, err error) {
	c, err = gl.Listener.Accept()
	if err != nil {
		return
	}

	c = gracefulConn{Conn: c}
	httpWg.Add(1)
	return
}

// Close the listener by stopping all requests from being processed
func (gl *gracefulListener) Close() error {
	if gl.stopped {
		return syscall.EINVAL
	}
	gl.stop <- nil
	return <-gl.stop
}

// File will return the file descriptor for the current tcp listener connection
func (gl *gracefulListener) File() *os.File {
	tl := gl.Listener.(*net.TCPListener)
	fl, _ := tl.File()
	return fl
}

// gracefulConn is a wrapped net.Conn closing with the wait sync group
type gracefulConn struct {
	net.Conn
}

// Close the current network connection
func (w gracefulConn) Close() error {
	httpWg.Done()
	return w.Conn.Close()
}

// These are here because there is no API in syscall for turning OFF
// close-on-exec (yet).

// from syscall/zsyscall_linux_386.go, but it seems like it might work
// for other platforms too.
func fcntl(fd int, cmd int, arg int) (val int, err error) {
	r0, _, e1 := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(cmd), uintptr(arg))
	val = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

func noCloseOnExec(fd uintptr) {
	fcntl(int(fd), syscall.F_SETFD, ^syscall.FD_CLOEXEC)
}
