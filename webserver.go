package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/nyxtom/workclient"
)

type WebServer struct {
	workclient.WorkClient
	cmdArgs        []string
	serverAddr     string
	cmd            string
	listenFD       int
	readTimeout    time.Duration
	writeTimeout   time.Duration
	maxHeaderBytes int
	listenerClosed chan error
	listener       *gracefulListener
}

func NewWebServer(config *workclient.Config, graceful bool, cmdArgs []string) *WebServer {
	server := new(WebServer)
	server.cmdArgs = cmdArgs
	server.serverAddr = config.WebAddr
	if graceful {
		server.listenFD = 3
	}
	server.maxHeaderBytes = config.MaxHeaderBytes
	server.readTimeout = config.ReadTimeout
	server.writeTimeout = config.WriteTimeout
	server.listenerClosed = make(chan error)
	server.Configure(config, server.listen, server.stopListening)
	server.Routes()
	return server
}

func (server *WebServer) Routes() {
	handle("/", logReq, hello)
	handle("/restart", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "restarting server\n")
		server.restartGraceful()
	})
	handle("/shutdown", func(w http.ResponseWriter, req *http.Request) {
		server.Close()
	})
}

func handle(path string, fns ...func(http.ResponseWriter, *http.Request)) {
	http.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		for _, fn := range fns {
			fn(w, req)
		}
	})
}

func logReq(w http.ResponseWriter, req *http.Request) {
	log.Printf("%v %v from %v", req.Method, req.URL, req.RemoteAddr)
}

func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello world\n")
}

func (server *WebServer) listen() {
	var err error
	var l net.Listener

	if server.listenFD != 0 {
		server.LogInfoF("Listening on existing file descriptor %d", server.listenFD)
		f := os.NewFile(uintptr(server.listenFD), "listen socket")
		l, err = net.FileListener(f)
	} else {
		server.LogInfo("Listening on a new file descriptor, " + server.serverAddr)
		l, err = net.Listen("tcp", server.serverAddr)
	}
	if err != nil {
		panic(err)
	}

	// attach sighup for restarting the server gracefully
	sc := make(chan os.Signal)
	signal.Notify(sc, syscall.SIGHUP)
	go func() {
		select {
		case <-sc:
			server.restartGraceful()
			return
		case err = <-server.listenerClosed:
			return
		}
	}()

	// tell the parent to stop accepting requests and exit
	if server.listenFD != 0 {
		parent := syscall.Getppid()
		server.LogInfoF("killing parent pid: %v", parent)
		syscall.Kill(parent, syscall.SIGTERM)
	}

	// setup the http server for the web server
	httpServer := &http.Server{
		Addr:           server.serverAddr,
		ReadTimeout:    server.readTimeout,
		WriteTimeout:   server.writeTimeout,
		MaxHeaderBytes: server.maxHeaderBytes}
	server.listener = newGracefulListener(l)
	httpServer.Serve(server.listener)
}

func (server *WebServer) restartGraceful() {
	server.LogInfo("initiated graceful restart for web server")
	fl := server.listener.File()
	args := []string{}
	for _, k := range server.cmdArgs[1:] {
		if k != "--graceful" {
			args = append(args, k)
		}
	}
	args = append(args, "--graceful")
	cmd := exec.Command(server.cmdArgs[0], args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{fl}
	err := cmd.Start()
	if err != nil {
		server.LogErr(err)
	}
}

func (server *WebServer) stopListening() {
	err := server.listener.Close()
	server.LogInfo("closing web server")
	if err != nil {
		server.LogErr(err)
	}
	server.listenerClosed <- err
}
