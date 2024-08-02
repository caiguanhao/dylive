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
	"text/template"
	"time"

	"github.com/caiguanhao/dylive"
)

var (
	currentRooms = map[string]string{}

	preferQuality, preferFormat string
	outputJson                  bool
	commadnTemplate             string
)

func main() {
	flag.StringVar(&preferQuality, "q", "", "video quality (uhd, hd, ld, sd)")
	flag.StringVar(&preferFormat, "f", "flv", "format (flv, hls, m3u8)")
	flag.BoolVar(&outputJson, "json", false, "output json instead of url")
	flag.StringVar(&commadnTemplate, "run", "", "command template to run; use @/path/to/template.sh to specify a template file")
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
			continue
		}
		currentRooms[id] = room.Id
		if room.StatusCode != dylive.RoomStatusLiveOn {
			log.Printf("%s (%s) hasn't started livestream yet.", room.User.Name, room.DouyinId)
			continue
		}
		log.Printf("%s (%s) is live.", room.User.Name, room.DouyinId)
		if preferFormat == "hls" || preferFormat == "m3u8" {
			room.StreamUrl = room.HlsUrlForQuality(preferQuality)
		} else {
			room.StreamUrl = room.FlvUrlForQuality(preferQuality)
		}
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
	err = tmpl.Execute(&cmdStrBuilder, room)
	if err != nil {
		return err
	}
	cmdStr := cmdStrBuilder.String()
	if cmdStr == "" {
		return nil
	}
	log.Println("Running command", cmdStr)
	cmd := exec.Command("sh", "-c", cmdStr)
	return cmd.Start()
}
