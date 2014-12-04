package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nyxtom/gracefulhttp"
	"github.com/nyxtom/workclient"
)

// WebServer is a simple work client enabled http server
type WebServer struct {
	workclient.WorkClient
	cmdArgs    []string
	closed     bool
	httpServer *gracefulhttp.Server
}

// NewWebServer returns a work client enabled http server
func NewWebServer(config *workclient.Config, fd int, cmdArgs []string) *WebServer {
	server := new(WebServer)
	server.cmdArgs = cmdArgs
	server.httpServer = gracefulhttp.NewServer(config.WebAddr, 0)
	server.httpServer.ReadTimeout = config.ReadTimeout
	server.httpServer.WriteTimeout = config.WriteTimeout
	server.httpServer.MaxHeaderBytes = config.MaxHeaderBytes
	server.httpServer.FileDescriptor = fd
	server.Configure(config, server.listen, server.stopListening)
	return server
}

// ServeWeb will create a web server, attach signal flags and run the worker
func ServeWeb(config *workclient.Config, fd int, cmdArgs []string) {
	server := NewWebServer(config, fd, cmdArgs)
	server.AttachSignals()
	server.Run()
}

// RestartGraceful will perform a no-downtime restart by passing off the socket to the forked process
func (server *WebServer) RestartGraceful() {
	server.LogInfo("initiated graceful restart for web server")
	fd := server.httpServer.Fd()
	args := []string{}
	for _, k := range server.cmdArgs[1:] {
		if !strings.Contains(k, "--fd=") {
			args = append(args, k)
		}
	}
	args = append(args, fmt.Sprintf("--fd=%d", fd))
	cmd := exec.Command(server.cmdArgs[0], args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		server.LogErr(err)
	}
}

// attachSignals will create a channel to OS.Signal to listen for any signup events..etc
func (server *WebServer) AttachSignals() {
	sc := make(chan os.Signal)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGKILL,
		os.Interrupt)

	go func() {
		for {
			signal := <-sc
			if signal == syscall.SIGHUP {
				server.RestartGraceful()
			} else {
				close(sc)
				server.Close()
				break
			}
		}
	}()
}

func (server *WebServer) listen() {
	if server.httpServer.FileDescriptor == 0 {
		server.LogInfoF("listening on %s", server.httpServer.Addr)
	} else {
		server.LogInfoF("listening on existing file descriptor %d, %s", server.httpServer.FileDescriptor, server.httpServer.Addr)
	}

	handleFunc("/", server.logReq, server.index)
	handleFunc("/restart", server.logReq, server.restart)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./public"))))

	err := server.httpServer.ListenAndServe()
	if err != nil {
		server.LogErr(err)
		server.Close()
	}
}

func (server *WebServer) stopListening() {
	server.httpServer.Close()
}

func (server *WebServer) logReq(w http.ResponseWriter, req *http.Request) {
	server.LogInfoF("%v %v from %v", req.Method, req.URL, req.RemoteAddr)
}

func (server *WebServer) index(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello world\n")
}

func (server *WebServer) restart(w http.ResponseWriter, req *http.Request) {
	server.RestartGraceful()
	http.Redirect(w, req, "/", http.StatusFound)
}

func (server *WebServer) shutdown(w http.ResponseWriter, req *http.Request) {
	server.Close()
}

// handleFunc takes a prefix and a list of http handlers to execute them as an in-order stack
func handleFunc(path string, fns ...func(http.ResponseWriter, *http.Request)) {
	http.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		for _, fn := range fns {
			fn(w, req)
		}
	})
}
