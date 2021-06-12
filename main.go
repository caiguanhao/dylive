package main

import (
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

func main() {
	deviceIdStr := flag.String("device", "66178590413", "device ID")
	durationStr := flag.String("duration", "5s",
		"check user live stream for every duration (ms, s, m, h), "+
			"must not be less than 1 second")
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

	duration, err := time.ParseDuration(*durationStr)
	if err != nil || duration.Seconds() < 1 {
		log.Fatalln("invalid duration")
	}

	deviceId, err := strconv.ParseUint(*deviceIdStr, 10, 64)
	if err != nil {
		log.Fatalln(err)
	}

	var text string
	if len(flag.Args()) == 0 {
		b, _ := ioutil.ReadAll(os.Stdin)
		text = string(b)
	} else {
		text = strings.Join(flag.Args(), "\n")
	}

	var userIds []uint64

	for {
		url, str := douyinapi.GetPageUrlStr(text)
		if url == "" {
			break
		}
		text = text[strings.Index(text, str)+len(str):]
		userId, roomId, _ := douyinapi.GetIdFromUrl(url)

		if userId > 0 {
			userIds = append(userIds, userId)
		} else if roomId > 0 {
			urlMap, err := douyinapi.GetLiveUrlFromRoomId(roomId)
			if err != nil {
				log.Println(err)
				continue
			}
			liveStreamUrl := getLiveStreamUrl(roomId, urlMap)
			fmt.Println(liveStreamUrl)
		}
	}

	if len(userIds) == 0 {
		return
	}

	names := map[uint64]string{}
	roomIds := map[uint64]uint64{}
	for {
		for _, userId := range userIds {
			user, err := douyinapi.GetUserInfo(deviceId, userId)
			if user != nil && names[user.Id] != user.Name {
				log.Println("checking live stream of user:", user.Id, user.Name)
				names[user.Id] = user.Name
			}
			if user == nil || user.RoomId == 0 {
				continue
			}
			if roomIds[user.Id] == user.RoomId {
				continue
			}
			urlMap, err := douyinapi.GetLiveUrlFromRoomId(user.RoomId)
			if err != nil {
				log.Println(err)
				continue
			}
			liveStreamUrl := getLiveStreamUrl(user.RoomId, urlMap)
			fmt.Println(liveStreamUrl)
			roomIds[userId] = user.RoomId
		}
		time.Sleep(duration)
	}
}

func getLiveStreamUrl(roomId uint64, urlMap map[string]string) (out string) {
	if url := urlMap["FULL_HD1"]; url != "" {
		out = url
	} else if url := urlMap["HD1"]; url != "" {
		out = url
	}
	for key, url := range urlMap {
		log.Println(roomId, key, url)
		if out == "" {
			out = url
		}
	}
	return
}
