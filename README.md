# webterm

Webterm is a simple web-based terminal implementation with simple backend commands
written in go-lang, built on top of the ace editor and jquery terminal. It's primarily
meant as a proof of concept terminal backed by a go-lang web server.

### webterm-broadcast

The server is backed using a [broadcast](http://github.com/nyxtom/broadcast) based
backend for communicating and handling various commands. Broadcast is a redis-alike
clone server which supports various communication protocols (line-by-line, redis or custom).
webterm-broadcast will communicate as quickly as possible using this method from any
number of clients. As a result, you can also access the server using **webterm-cli**

```
nyxtom@higgs$ webterm-broadcast
[3571] 27 Jul 15 00:37 CDT # WARNING: no config file specified, using the default config
[3571] 27 Jul 15 00:37 CDT #

             __   __
 _    _____ / /  / /____ ______ _     WebTerm 0.1.0 64 bit
| |/|/ / -_) _ \/ __/ -_) __/  ' \    Port: 7337
|__,__/\__/_.__/\__/\__/_/ /_/_/_/    PID: 3571


[3571] 27 Jul 15 00:37 CDT # setting read/write protocol to redis
[3571] 27 Jul 15 00:37 CDT # listening for incoming connections on 127.0.0.1:7337
```

Commands available from the cli is exactly how the web terminal behaves. You can run
the cli using the command below to test it out.

```
nyxtom@higgs$ webterm-cli
127.0.0.1:7337> cmds
CAT
 Concatenate the contents of a file

CMDS
 List of available commands supported by the server

DIR
 Lists the files in the directory

ECHO
 Echos back a message sent
 usage: ECHO "hello world"

EDIT
 Edit the contents of a file

INFO
 Current server status and information

LS
 Lists the files in the directory

PING
 Pings the server for a response

SAVE
 Saves the contents of a file

127.0.0.1:7337>
```

## webterm web server

**webterm** is our actual web server and this command is run from within the main
directory of the repository. You can manually build this or run the command where the
**app** directory sits. The result is a simple web based cli. The web-server is written
in go-lang and leverages a few utilities I wrote including [workclient](http://github.com/nyxtom/workclient) (a
service wrapper allowing you to configure the server to etcd, statsd...etc). 

### Licence

The MIT License (MIT)

Copyright (c) 2014 Thomas Holloway

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
