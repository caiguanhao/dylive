package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/caiguanhao/dylive/douyinapi"
)

var (
	DefaultDeviceId = douyinapi.DefaultDeviceId
)

func main() {
	numberOfDeviceIds := flag.Int("n", 0, "enumerate and print working device ids "+
		"starting from -device until number of ids are found")
	disableAutoGetOne := flag.Bool("N", false, "exit if device is not working, "+
		"do not try to get one automatically")
	deviceIdStr := flag.String("device", strconv.FormatUint(DefaultDeviceId, 10),
		"try to use this device ID and then the default one")
	durationStr := flag.String("duration", "5s",
		"check user live stream for every duration (ms, s, m, h), "+
			"must not be less than 1 second")
	jsonOutput := flag.Bool("json", false, "standard output uses json")
	verbose := flag.Bool("verbose", false, "verbosive")
	flag.Usage = func() {
		fmt.Println("Usage of dylive [OPTIONS] [URL|ID]")
		fmt.Println(`
  This utility reads Douyin's share URLs from standard input (if no arguments
  provided) and writes live stream URLs (.m3u8) to standard output.

  In any live stream room, click "Share" and copy the share message.

  In any user profile page, click "Share" and copy the share message.
  Once user starts new live stream room, URL is written.

  Example:

    dylive exJ1CqY exJk92q | xargs -n 1 open -na mpv`)
		fmt.Println()
		flag.PrintDefaults()
	}
	flag.Parse()

	douyinapi.Verbose = *verbose

	deviceId, err := strconv.ParseUint(*deviceIdStr, 10, 64)
	if err != nil {
		log.Fatalln(err)
	}

	if *numberOfDeviceIds > 0 {
		enumerateDeviceId(deviceId, *numberOfDeviceIds, true)
		return
	}

	duration, err := time.ParseDuration(*durationStr)
	if err != nil || duration.Seconds() < 1 {
		log.Fatalln("invalid duration")
	}

	var text string
	if len(flag.Args()) == 0 {
		b, _ := ioutil.ReadAll(os.Stdin)
		text = string(b)
	} else {
		text = strings.Join(flag.Args(), "\n")
	}

	var userIds []uint64
	userMap := map[uint64]string{}

	for {
		url, str := douyinapi.GetPageUrlStr(text)
		if url == "" {
			break
		}
		text = text[strings.Index(text, str)+len(str):]
		userId, roomId, _ := douyinapi.GetIdFromUrl(url)

		if userId > 0 {
			userIds = append(userIds, userId)
			userMap[userId] = str
		} else if roomId > 0 {
			room, err := douyinapi.GetRoom(roomId)
			if err != nil {
				log.Println(err)
				continue
			}
			liveStreamUrl := getLiveStreamUrl(room, !*jsonOutput)
			if *jsonOutput {
				json.NewEncoder(os.Stdout).Encode(struct {
					*douyinapi.Room
					LiveStreamUrl string
				}{room, liveStreamUrl})
			} else {
				fmt.Println(liveStreamUrl)
			}
		}
	}

	if len(userIds) == 0 {
		return
	}

	names := map[uint64]string{}
	roomIds := map[uint64]uint64{}
	failed := 0
	defaultTried := deviceId == DefaultDeviceId
outer:
	for {
		for _, userId := range userIds {
			user, err := douyinapi.GetUserInfo(deviceId, userId)
			if user != nil && names[user.Id] != user.Name {
				log.Printf("checking live stream of user: %d (%s) %s for every %s",
					user.Id, userMap[user.Id], user.Name, duration)
				names[user.Id] = user.Name
			}
			if user == nil {
				failed += 1
				if failed > 1 {
					if !defaultTried {
						log.Printf("device id %d is not working, "+
							"trying the default one", deviceId)
						deviceId = DefaultDeviceId
						defaultTried = true
					} else if *disableAutoGetOne {
						log.Fatalf("fatal: device id %d is not working, "+
							`you can use "dylive -n 1" to get one`, deviceId)
					} else {
						log.Printf("device id %d is not working, trying new device id", deviceId)
						deviceIds := enumerateDeviceId(deviceId, 1, false)
						deviceId = deviceIds[0]
						log.Printf("you should update your command like this: "+
							`"alias dylive='dylive -device %d'"`, deviceId)
					}
				}
				time.Sleep(1 * time.Second)
				continue outer
			}
			failed = 0
			if user.RoomId == 0 {
				continue
			}
			if roomIds[user.Id] == user.RoomId {
				continue
			}
			room, err := douyinapi.GetRoom(user.RoomId)
			if err != nil {
				log.Println(err)
				continue
			}
			liveStreamUrl := getLiveStreamUrl(room, !*jsonOutput)
			if *jsonOutput {
				room.User = user
				json.NewEncoder(os.Stdout).Encode(struct {
					*douyinapi.Room
					LiveStreamUrl string
				}{room, liveStreamUrl})
			} else {
				fmt.Println(liveStreamUrl)
			}
			roomIds[userId] = user.RoomId
		}
		time.Sleep(duration)
	}
}

func getLiveStreamUrl(room *douyinapi.Room, printUrl bool) (out string) {
	if url := room.StreamHlsUrlMap["FULL_HD1"]; url != "" {
		out = url
	} else if url := room.StreamHlsUrlMap["HD1"]; url != "" {
		out = url
	}
	for key, url := range room.StreamHlsUrlMap {
		if printUrl {
			log.Println(room.Id, key, url)
		}
		if out == "" {
			out = url
		}
	}
	return
}

func enumerateDeviceId(from uint64, count int, printId bool) (out []uint64) {
	var i uint64
	var found int
	for found < count {
		id := from + i
		log.Println("checking", id)
		user, _ := douyinapi.GetUserInfo(id, 2128250633728555)
		if user != nil {
			log.Println("found working device id", id)
			if printId {
				fmt.Println(id)
			}
			out = append(out, id)
			found += 1
		}
		i += 1
	}
	return
}
