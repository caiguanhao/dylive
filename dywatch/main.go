package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/caiguanhao/dylive"
)

var (
	currentRooms = map[string]string{}
	pids         = map[string]int{}

	preferQuality, preferFormat string
	outputJson                  bool
	commadnTemplate             string
	checkCommand                bool
)

func main() {
	flag.StringVar(&preferQuality, "q", "", "video quality (uhd, hd, ld, sd)")
	flag.StringVar(&preferFormat, "f", "flv", "format (flv, hls, m3u8)")
	flag.BoolVar(&outputJson, "json", false, "output json instead of url")
	flag.StringVar(&commadnTemplate, "run", "", "command template to run; use @/path/to/template.sh to specify a template file")
	flag.BoolVar(&checkCommand, "check", false, "re-run command if process does not exist")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Monitor live streams from Douyin.")
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	for {
		getRoom()
		time.Sleep(5 * time.Second)
	}
}

func getRoom() {
	ids := flag.Args()
	if len(ids) == 0 {
		log.Println("At least one Douyin ID is required.")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range ids {
		room, err := dylive.GetRoom(ctx, id)
		if err != nil {
			log.Println(err)
			continue
		}
		if currentRooms[id] == room.Id {
			if checkCommand && room.StatusCode == dylive.RoomStatusLiveOn && pids[room.Id] > 0 && !isProcessRunning(pids[room.Id]) {
				log.Println("Process", pids[room.Id], "exited, restart")
				updateStreamUrl(room)
				if err := runCommand(commadnTemplate, room); err != nil {
					log.Println(err)
				}
			}
			continue
		}
		currentRooms[id] = room.Id
		if room.StatusCode != dylive.RoomStatusLiveOn {
			log.Printf("%s (%s) hasn't started livestream yet.", room.User.Name, room.DouyinId)
			continue
		}
		log.Printf("%s (%s) is live.", room.User.Name, room.DouyinId)
		updateStreamUrl(room)
		if outputJson {
			json.NewEncoder(os.Stdout).Encode(room)
		} else {
			fmt.Println(room.StreamUrl)
		}
		if commadnTemplate != "" {
			if err := runCommand(commadnTemplate, room); err != nil {
				log.Println(err)
			}
		}
	}
}

func updateStreamUrl(room *dylive.Room) {
	if preferFormat == "hls" || preferFormat == "m3u8" {
		room.StreamUrl = room.HlsUrlForQuality(preferQuality)
	} else {
		room.StreamUrl = room.FlvUrlForQuality(preferQuality)
	}
}

func runCommand(tpl string, room *dylive.Room) error {
	if len(tpl) > 1 && strings.HasPrefix(tpl, "@") {
		content, _ := os.ReadFile(tpl[1:])
		tpl = string(content)
	}
	if tpl == "" {
		return nil
	}
	tmpl, err := template.New("").Parse(tpl)
	if err != nil {
		return err
	}
	var cmdStrBuilder strings.Builder
	err = tmpl.Execute(&cmdStrBuilder, struct {
		*dylive.Room
		Timestamp int64
	}{
		Room:      room,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	cmdStr := cmdStrBuilder.String()
	if cmdStr == "" {
		return nil
	}
	cmd := exec.Command("sh", "-c", cmdStr)
	err = cmd.Start()
	if err == nil {
		log.Println("Command", cmdStr, "started as PID", cmd.Process.Pid)
		pids[room.Id] = cmd.Process.Pid
		go func() {
			err := cmd.Wait()
			if err != nil {
				log.Printf("Process %d exited with error: %s\n", cmd.Process.Pid, err)
			} else {
				log.Printf("Process %d exited successfully\n", cmd.Process.Pid)
			}
		}()
	}
	return err
}

func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}
	return true
}
