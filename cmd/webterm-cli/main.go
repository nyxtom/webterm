package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nyxtom/broadcast/client/go/broadcast"
)

var helpCommands = [][]string{}
var helpCommandsMap = make(map[string]int)

func main() {
	var ip = flag.String("h", "127.0.0.1", "webterm server ip (default 127.0.0.1)")
	var port = flag.Int("p", 7337, "webterm server port (default 7331)")
	var bprotocol = flag.String("bprotocol", "redis", "broadcast server protocol to follow")
	var maxIdle = flag.Int("i", 1, "max idle client connections to pool from")

	flag.Parse()

	addr := *ip + ":" + strconv.Itoa(*port)
	c, err := broadcast.NewClient(*port, *ip, *maxIdle, *bprotocol)
	if err != nil {
		fmt.Printf(err.Error())
		os.Exit(1)
	}

	reply, err := c.Do("cmds")
	if err != nil {
		fmt.Printf("%s", err.Error())
	} else {
		printReply("cmds", reply, "")
	}

	SetCompletionHandler(completionHandler)
	setHistoryCapacity(100)

	reg, _ := regexp.Compile(`'.*?'|".*?"|\S+`)
	prompt := ""

	for {
		prompt = fmt.Sprintf("%s> ", addr)

		cmd, err := line(prompt)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			return
		}

		cmds := reg.FindAllString(cmd, -1)
		if len(cmds) == 0 {
			continue
		} else {
			addHistory(cmd)

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
			if strings.ToLower(cmd) == "help" || cmd == "?" {
				printHelp(cmds)
			} else if cmd == "CMDS" {
				printCmds()
			} else {
				async := isCmdAsync(cmd)
				if async {
					c.DoAsync(cmd, args...)
				} else {
					reply, err := c.Do(cmd, args...)
					if err != nil {
						fmt.Printf("%s", err.Error())
					} else {
						printReply(cmd, reply, "")
					}
				}
				fmt.Printf("\n")
			}
		}
	}
}

func isCmdAsync(cmd string) bool {
	for _, v := range helpCommands {
		if v[0] == cmd && v[3] == "true" {
			return true
		}
	}
	return false
}

func printReply(cmd string, reply interface{}, indent string) {
	if strings.ToLower(cmd) == "cmds" {
		r, ok := reply.(map[string]interface{})
		if !ok {
			fmt.Printf("%s\n", string(reply.(error).Error()))
			return
		}
		helpReply := false
		if helpCommands == nil || len(helpCommands) == 0 {
			helpReply = true
			helpCommands = make([][]string, 0)
		}
		for k, v := range r {
			cmd := v.(map[string]interface{})
			desc := cmd["Description"].(string)
			usage := cmd["Usage"].(string)
			async := fmt.Sprintf("%v", (cmd["FireForget"].(bool)))
			if helpReply {
				helpCommands = append(helpCommands, []string{k, usage, desc, async})
				helpCommandsMap[k] = len(helpCommands) - 1
			} else {
				printCommandHelp([]string{k, usage, desc, async})
			}
		}
		return
	}
	switch reply := reply.(type) {
	case int64:
		fmt.Printf("(integer) %d\n", reply)
	case float64:
		fmt.Printf("(float) %f\n", reply)
	case string:
		fmt.Printf("%s\n", reply)
	case []byte:
		fmt.Printf("%q\n", reply)
	case nil:
		fmt.Printf("(nil)\n")
	case bool:
		fmt.Printf("%v\n", reply)
	case error:
		fmt.Printf("%s\n", string(reply.Error()))
	case map[string]interface{}:
		mk := make([]string, len(reply))
		i := 0
		for k, _ := range reply {
			mk[i] = k
			i++
		}
		sort.Strings(mk)
		for _, v := range mk {
			replyV := reply[v]
			if replyV != nil {
				fmt.Printf(indent+"%s-> ", v)
				if _, ok := replyV.([]interface{}); ok {
					fmt.Printf("\n")
				}
				if _, ok := replyV.(map[string]interface{}); ok {
					fmt.Printf("\n")
				}
				printReply(cmd, replyV, indent+"  ")
			}
		}
	case []interface{}:
		if len(reply) > 10 {
			reply = reply[:10]
		}
		for i, v := range reply {
			if _, ok := v.(map[string]interface{}); ok {
				fmt.Printf(indent+"%d) \n", i+1)
			} else {
				fmt.Printf(indent+"%d) ", i+1)
			}
			printReply(cmd, v, indent+"  ")
		}
	}
}

func printGenericHelp() {
	msg :=
		`broadcast-cli
Type:   "help <command>" for help on <command>
    `
	fmt.Println(msg)
}

func printCommandHelp(arr []string) {
	fmt.Printf("%s\n %s", arr[0], arr[2])
	if len(arr[1]) > 0 {
		fmt.Printf("\n usage: %s", arr[1])
	}
	fmt.Printf("\n\n")
}

func printCmds() {
	var cmds []string
	for _, v := range helpCommands {
		cmds = append(cmds, v[0])
	}

	sort.Strings(cmds)
	for _, v := range cmds {
		i := helpCommandsMap[v]
		printCommandHelp(helpCommands[i])
	}
}

func printHelp(cmds []string) {
	args := cmds[1:]
	if len(args) == 0 {
		printGenericHelp()
	} else if len(args) > 1 {
		fmt.Println()
	} else {
		cmd := strings.ToUpper(args[0])
		for i := 0; i < len(helpCommands); i++ {
			if helpCommands[i][0] == cmd {
				printCommandHelp(helpCommands[i])
			}
		}
	}
}

func completionHandler(in string) []string {
	var keywords []string
	for _, i := range helpCommands {
		if strings.HasPrefix(i[0], strings.ToUpper(in)) {
			keywords = append(keywords, i[0])
		}
	}
	return keywords
}
