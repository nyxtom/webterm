package webterm

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

type WebServer struct {
	WorkClient
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

func NewWebServer(config *AppConfiguration, graceful bool, cmdArgs []string) *WebServer {
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
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "hello world\n")
	})
	http.HandleFunc("/restart", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "restarting server\n")
		server.restartGraceful()
	})
	http.HandleFunc("/shutdown", func(w http.ResponseWriter, req *http.Request) {
		server.Close()
	})
	return server
}

func (server *WebServer) listen() {
	var err error
	var l net.Listener

	if server.listenFD != 0 {
		server.events <- LogEvent{"info", fmt.Sprintf("Listening on existing file descriptor %d", server.listenFD), nil, nil}
		f := os.NewFile(uintptr(server.listenFD), "listen socket")
		l, err = net.FileListener(f)
	} else {
		server.events <- LogEvent{"info", "Listening on a new file descriptor, " + server.serverAddr, nil, nil}
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
		server.events <- LogEvent{"info", fmt.Sprintf("killing parent pid: %v", parent), nil, nil}
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
	server.events <- LogEvent{"info", "initiated graceful restart for web server", nil, nil}
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
		server.events <- LogEvent{"error", "", err, nil}
	}
}

func (server *WebServer) stopListening() {
	err := server.listener.Close()
	server.events <- LogEvent{"info", "closing web server", nil, nil}
	if err != nil {
		server.events <- LogEvent{"error", "", err, nil}
	}
	server.listenerClosed <- err
}
