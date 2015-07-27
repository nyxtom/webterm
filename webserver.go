package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/nyxtom/broadcast/client/go/broadcast"
	"github.com/nyxtom/gracefulhttp"
	"github.com/nyxtom/workclient"
)

// WebServer is a simple work client enabled http server
type WebServer struct {
	workclient.WorkClient
	cmdArgs    []string
	closed     bool
	httpServer *gracefulhttp.Server
	bport      int
	bip        string
	bprotocol  string
}

type WebConfig struct {
	workclient.Config

	// broadcast configuration
	BroadcastPort  int    `toml:"broadcast_port" default:"7337"`
	BroadcastIP    string `toml:"broadcast_ip" default:"127.0.0.1"`
	BroadcastProto string `toml:"broadcast_proto" default:"redis"`
}

// NewWebServer returns a work client enabled http server
func NewWebServer(config *WebConfig, fd int, cmdArgs []string) *WebServer {
	server := new(WebServer)
	server.cmdArgs = cmdArgs
	server.httpServer = gracefulhttp.NewServer(config.WebAddr, 0)
	server.httpServer.ReadTimeout = config.ReadTimeout
	server.httpServer.WriteTimeout = config.WriteTimeout
	server.httpServer.MaxHeaderBytes = config.MaxHeaderBytes
	server.httpServer.FileDescriptor = fd
	server.Configure(config.Config, server.listen, server.stopListening)
	server.bport = config.BroadcastPort
	server.bip = config.BroadcastIP
	server.bprotocol = config.BroadcastProto
	return server
}

// ServeWeb will create a web server, attach signal flags and run the worker
func ServeWeb(config *WebConfig, fd int, cmdArgs []string) {
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

// AttachSignals will create a channel to OS.Signal to listen for any signup events..etc
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

	handleFunc("/exec", server.logReq, server.exec)
	handleFunc("/", server.logReq, server.index)
	//handleFunc("/restart", server.logReq, server.restart)
	//handleFunc("/shutdown", server.logReq, server.shutdown)
	items := []string{"scripts", "styles", "fonts", "static"}
	for _, k := range items {
		prefix := "/" + k + "/"
		dir := path.Join("./app", k)
		http.Handle(prefix, http.StripPrefix(prefix, http.FileServer(http.Dir(dir))))
	}

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
	t, _ := template.ParseFiles(path.Join("./app", "index.html"))
	t.Execute(w, nil)
}

func (server *WebServer) exec(w http.ResponseWriter, req *http.Request) {
	values := req.URL.Query()
	response := make(map[string]interface{})
	if len(values["cmd"]) > 0 {
		c, err := broadcast.NewClient(server.bport, server.bip, 1, server.bprotocol)
		if err != nil {
			server.LogErr(err)
			return
		}
		reg, _ := regexp.Compile(`'.*?'|".*?"|\S+`)
		cmds := reg.FindAllString(values["cmd"][0], -1)
		if len(cmds) > 0 {
			args := make([]interface{}, len(cmds[1:]))
			for i := range args {
				item := strings.Trim(string(cmds[1+i]), "\"'")
				if a, err := strconv.Atoi(item); err == nil {
					args[i] = a
				} else if a, err := strconv.ParseFloat(item, 64); err == nil {
					args[i] = a
				} else if a, err := strconv.ParseBool(item); err == nil {
					args[i] = a
				} else if len(item) == 1 {
					b := []byte(item)
					args[i] = string(b[0])
				} else {
					args[i] = item
				}
			}
			cmd := strings.ToUpper(cmds[0])
			reply, err := c.Do(cmd, args...)
			if err != nil {
				server.LogErr(err)
			} else {
				response["cmd"] = cmd
				response["args"] = args
				response["reply"] = printReply(cmd, reply, "")
			}
		}
	}

	js, err := json.Marshal(response)
	if err != nil {
		server.LogErr(err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func printReply(cmd string, reply interface{}, indent string) interface{} {
	switch reply := reply.(type) {
	case []byte:
		return fmt.Sprintf("%q\n", reply)
	case nil:
		return fmt.Sprintf("(nil)\n")
	case error:
		return fmt.Sprintf("%s\n", string(reply.Error()))
	}

	return reply
}

/*
func (server *WebServer) restart(w http.ResponseWriter, req *http.Request) {
	server.RestartGraceful()
	http.Redirect(w, req, "/", http.StatusFound)
}

func (server *WebServer) shutdown(w http.ResponseWriter, req *http.Request) {
	server.Close()
}
*/

// handleFunc takes a prefix and a list of http handlers to execute them as an in-order stack
func handleFunc(path string, fns ...func(http.ResponseWriter, *http.Request)) {
	http.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		for _, fn := range fns {
			fn(w, req)
		}
	})
}
