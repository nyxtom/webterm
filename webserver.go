package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/nyxtom/gracefulhttp"
	"github.com/nyxtom/workclient"
)

type WebServer struct {
	workclient.WorkClient
	cmdArgs        []string
	listenerClosed chan error
	httpServer     *gracefulhttp.Server
}

func NewWebServer(config *workclient.Config, graceful bool, cmdArgs []string) *WebServer {
	server := new(WebServer)
	server.cmdArgs = cmdArgs
	server.listenerClosed = make(chan error)
	server.httpServer = gracefulhttp.NewServer(config.WebAddr, 0)
	server.httpServer.ReadTimeout = config.ReadTimeout
	server.httpServer.WriteTimeout = config.WriteTimeout
	server.httpServer.MaxHeaderBytes = config.MaxHeaderBytes
	if graceful {
		server.httpServer.FileDescriptor = 3
	}
	server.Configure(config, server.listen, server.stopListening)
	return server
}

func handleFunc(path string, fns ...func(http.ResponseWriter, *http.Request)) {
	http.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		for _, fn := range fns {
			fn(w, req)
		}
	})
}

func (server *WebServer) logReq(w http.ResponseWriter, req *http.Request) {
	server.LogInfoF("%v %v from %v", req.Method, req.URL, req.RemoteAddr)
}

func (server *WebServer) hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello world\n")
}

func (server *WebServer) listen() {
	// attach sighup for restarting the server gracefully
	sc := make(chan os.Signal)
	signal.Notify(sc, syscall.SIGHUP)
	go func() {
		select {
		case <-sc:
			server.restartGraceful()
			return
		case <-server.listenerClosed:
			return
		}
	}()

	if server.httpServer.FileDescriptor == 0 {
		server.LogInfoF("listening on %s", server.httpServer.Addr)
	} else {
		server.LogInfoF("listening on existing file descriptor %d, %s", server.httpServer.FileDescriptor, server.httpServer.Addr)
	}

	handleFunc("/", server.logReq, server.hello)
	handleFunc("/restart", server.logReq, func(w http.ResponseWriter, req *http.Request) {
		server.restartGraceful()
	})
	handleFunc("/shutdown", server.logReq, func(w http.ResponseWriter, req *http.Request) {
		server.Close()
	})

	server.httpServer.ListenAndServe()
}

func (server *WebServer) restartGraceful() {
	server.LogInfo("initiated graceful restart for web server")
	fl := server.httpServer.File()
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
	err := server.httpServer.Close()
	server.LogInfo("closing web server")
	if err != nil {
		server.LogErr(err)
	}
	server.listenerClosed <- err
}
