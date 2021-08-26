package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/caiguanhao/dylive/douyinapi"
)

func main() {
	durationStr := flag.String("duration", "5s",
		"check user live stream for every duration (ms, s, m, h),\n"+
			"must not be less than 1 second")
	jsonOutput := flag.Bool("json", false, "standard output uses json")
	executeCommand := flag.String("exec", "", "command to execute for every live stream,\n"+
		"use -json to see list of usable variables like {{.LiveStreamUrl}}")
	flag.Usage = func() {
		fmt.Println("Usage of dylive [OPTIONS] [USER-ID...]")
		fmt.Println(`
This utility prints Douyin user live stream room info to standard output.
The Douyin user ID (or user name) is listed below the user's nick name in the
user profile page.

EXAMPLE:

    dylive hongjingzhibo 1011694538 | xargs -n1 open -na mpv

OPTIONS:`)
		fmt.Println()
		flag.PrintDefaults()
	}
	flag.Parse()

	var commandTemplate *template.Template
	if *executeCommand != "" {
		var err error
		commandTemplate, err = template.New("").Parse(*executeCommand)
		if err != nil {
			log.Fatalln(err)
		}
	}

	duration, err := time.ParseDuration(*durationStr)
	if err != nil || duration.Seconds() < 1 {
		log.Fatalln("invalid duration")
	}

	if len(flag.Args()) == 0 {
		log.Fatalln("need at least one user name")
	}

	started := map[string]bool{}
	roomIds := map[string]uint64{}
	for {
		for _, name := range flag.Args() {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			user, err := douyinapi.GetUserByName(name)
			if err != nil {
				log.Println(name+":", err)
				continue
			}
			userId := user.SecUid
			if !started[userId] {
				log.Printf("checking live stream of user: %s (%s) for every %s",
					user.NickName, user.Name, duration)
				started[userId] = true
			}
			if user.Room == nil || user.Room.Operating == false {
				continue
			}
			roomId := uint64(user.Room.Id)
			if roomIds[userId] == roomId {
				continue
			}
			output(user, *jsonOutput, commandTemplate)
			roomIds[userId] = roomId
		}
		time.Sleep(duration)
	}
}

var cmdIdx int = -1

func output(user *douyinapi.User, usesJson bool, commandTemplate *template.Template) {
	cmdIdx += 1
	var liveStreamUrl string
	if user.Room.StreamHlsUrlMap == nil {
		room, _ := douyinapi.GetRoom(user.Room.PageUrl)
		user.Room = room
	}
	if url := user.Room.StreamHlsUrlMap["FULL_HD1"]; url != "" {
		liveStreamUrl = url
	} else if url := user.Room.StreamHlsUrlMap["HD1"]; url != "" {
		liveStreamUrl = url
	}
	for key, url := range user.Room.StreamHlsUrlMap {
		if !usesJson {
			log.Println(user.Room.Id, key, url)
		}
		if liveStreamUrl == "" {
			liveStreamUrl = url
		}
	}
	obj := struct {
		*douyinapi.User
		LiveStreamUrl string
		Index         int
	}{user, liveStreamUrl, cmdIdx}
	if usesJson {
		json.NewEncoder(os.Stdout).Encode(obj)
	} else {
		fmt.Println(liveStreamUrl)
	}
	if commandTemplate == nil {
		return
	}
	var buf bytes.Buffer
	err := commandTemplate.Execute(&buf, obj)
	if err != nil {
		log.Println(err)
		return
	}
	parts, err := strToArgv(buf.String())
	if err != nil {
		log.Println(err)
		return
	}
	if len(parts) == 0 {
		return
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	b, _ := json.Marshal(obj)
	cmd.Stdin = bytes.NewReader(append(b, '\n'))
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	log.Printf("cmd#%02d starting: %s", cmdIdx, cmd)
	err = cmd.Start()
	if err != nil {
		log.Printf("cmd#%02d error: %s", cmdIdx, err)
		return
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("cmd#%02d error: %s", cmdIdx, err)
		} else {
			log.Printf("cmd#%02d ran successfully", cmdIdx)
		}
	}()
}

// copied from https://github.com/cloverstd/parse-string-argv
func strToArgv(cmd string) (argv []string, err error) {
	const (
		singleQuote = '\''
		doubleQuote = '"'
		split       = ' '
	)
	var (
		temp      []byte
		prevQuote byte
	)
	for i := 0; i < len(cmd); i++ {
		switch cmd[i] {
		case split:
			if prevQuote == 0 {
				if len(temp) != 0 {
					argv = append(argv, string(temp))
					temp = temp[:0]
				}
				continue // skip space
			}
		case singleQuote, doubleQuote:
			if prevQuote == 0 {
				if i == 0 || cmd[i-1] == split {
					prevQuote = cmd[i]
					continue
				}
			} else if cmd[i] == prevQuote {
				if i == len(cmd)-1 {
					if len(temp) != 0 {
						argv = append(argv, string(temp))
						temp = temp[:0]
					}
				} else if cmd[i+1] != split {
					argv = argv[:0]
					return nil, fmt.Errorf("invalid cmd string: %s", cmd)
				}
				prevQuote = 0
				if len(temp) != 0 {
					argv = append(argv, string(temp))
					temp = temp[:0]
				}
				continue
			}
		}
		temp = append(temp, cmd[i])
		if len(cmd)-1 == i {
			argv = append(argv, string(temp))
			temp = temp[:0]
		}
	}
	if prevQuote != 0 {
		err = errors.New("invalid cmd string: unclosed quote")
		argv = argv[:0]
	}
	return
}
