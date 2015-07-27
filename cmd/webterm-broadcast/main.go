package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nyxtom/broadcast/backends/bdefault"
	"github.com/nyxtom/broadcast/protocols/line"
	"github.com/nyxtom/broadcast/protocols/redis"
	"github.com/nyxtom/broadcast/server"
)

type Configuration struct {
	port      int    // port of the server
	host      string // host of the server
	bprotocol string // broadcast protocol configuration
}

var LogoHeader = `

             __   __
 _    _____ / /  / /____ ______ _     %s %s %s
| |/|/ / -_) _ \/ __/ -_) __/  ' \    Port: %d
|__,__/\__/_.__/\__/\__/_/ /_/_/_/    PID: %d

`

func main() {
	// Leverage all cores available
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Parse out flag parameters
	var host = flag.String("h", "127.0.0.1", "webterm host to bind to")
	var port = flag.Int("p", 7337, "webterm port to bind to")
	var bprotocol = flag.String("bprotocol", "redis", "Broadcast protocol configuration")
	var configFile = flag.String("config", "", "webterm configuration file (/etc/webterm.conf)")
	var cpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")
	var homedir = flag.String("homedir", "", "home directory to serve static files")

	flag.Parse()

	cfg := &Configuration{*port, *host, *bprotocol}
	if len(*configFile) == 0 {
		fmt.Printf("[%d] %s # WARNING: no config file specified, using the default config\n", os.Getpid(), time.Now().Format(time.RFC822))
	} else {
		data, err := ioutil.ReadFile(*configFile)
		if err != nil {
			fmt.Println(err)
			return
		}
		_, err = toml.Decode(string(data), cfg)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// locate the protocol specified (if there is one)
	var serverProtocol server.BroadcastServerProtocol
	if cfg.bprotocol == "" {
		serverProtocol = server.NewDefaultBroadcastServerProtocol()
	} else if cfg.bprotocol == "redis" {
		serverProtocol = redisProtocol.NewRedisProtocol()
	} else if cfg.bprotocol == "line" {
		serverProtocol = lineProtocol.NewLineProtocol()
	} else {
		fmt.Println(errors.New("Invalid protocol " + cfg.bprotocol + " specified"))
		return
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Println(err)
			return
		}
		pprof.StartCPUProfile(f)
	}

	// create a new broadcast server
	app, err := server.ListenProtocol(cfg.port, cfg.host, serverProtocol)
	app.Header = ""
	app.Name = "WebTerm"
	app.Version = "0.1.0"
	app.Header = LogoHeader
	if err != nil {
		fmt.Println(err)
		return
	}

	// setup default backend
	backend, err := bdefault.RegisterBackend(app)
	if err != nil {
		fmt.Println(err)
		return
	}
	app.LoadBackend(backend)

	// setup bgraph backend
	backend, err = RegisterTermBackend(app, *homedir)
	if err != nil {
		fmt.Println(err)
		return
	}
	app.LoadBackend(backend)

	// wait for all events to fire so we can log them
	pid := os.Getpid()
	go func() {
		for !app.Closed {
			event := <-app.Events
			t := time.Now()
			delim := "#"
			if event.Level == "error" {
				delim = "ERROR:"
			}
			msg := fmt.Sprintf("[%d] %s %s %s", pid, t.Format(time.RFC822), delim, event.Message)
			if event.Err != nil {
				msg += fmt.Sprintf(" %v", event.Err)
			}

			fmt.Println(msg)
		}
	}()

	go func() {
		<-app.Quit
		pprof.StopCPUProfile()
		os.Exit(0)
	}()

	// attach to any signals that would cause our app to close
	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		os.Interrupt)

	go func() {
		<-sc
		app.Close()
	}()

	// accept incomming connections!
	app.AcceptConnections()
}
