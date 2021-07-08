package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/caiguanhao/dylive/douyinapi"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"
)

var (
	verbosive  bool
	wsDebugUrl string
)

type (
	Id uint64

	User struct {
		Id             Id
		UniqueId       Id
		SecUid         string
		Name           string
		NickName       string
		Description    string
		Picture        string
		FollowersCount int
		FavoritedCount int
		Room           *douyinapi.Room
	}

	byUserFollowersCount []User

	response struct {
		UserList []struct {
			UserInfo struct {
				UID         string `json:"uid"`
				ShortID     string `json:"short_id"`
				Nickname    string `json:"nickname"`
				Signature   string `json:"signature"`
				AvatarThumb struct {
					URLList []string `json:"url_list"`
				} `json:"avatar_thumb"`
				FollowerCount  int    `json:"follower_count"`
				TotalFavorited int    `json:"total_favorited"`
				UniqueID       string `json:"unique_id"`
				SecUID         string `json:"sec_uid"`
			} `json:"user_info"`
		} `json:"user_list"`
	}
)

func (a byUserFollowersCount) Len() int           { return len(a) }
func (a byUserFollowersCount) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byUserFollowersCount) Less(i, j int) bool { return a[i].FollowersCount > a[j].FollowersCount }

func (id Id) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(id), 10))
}

func main() {
	flag.BoolVar(&verbosive, "v", false, "verbosive")
	flag.StringVar(&wsDebugUrl, "ws", "", "WebSocket debugger URL")
	minFollowers := flag.Int("f", 1000, "only list users having at least number of followers")
	maxUsers := flag.Int("n", 0, "get at most number of users for every query, 0 to disable")
	useJson := flag.Bool("json", false, "output json")
	useTable := flag.Bool("table", false, "output table")
	sortByFollowers := flag.Bool("F", false, "sort by followers count")
	getLiveInfo := flag.Bool("l", false, "also get user live room info")
	showOnlyLive := flag.Bool("L", false, "only list users started live broadcast")
	flag.Parse()

	allUsers := []User{}

	for _, arg := range flag.Args() {
		data, err := getResponse(arg)
		if err != nil {
			log.Println(arg+":", err)
			continue
		}
		users, err := getUsers(data)
		if err != nil {
			log.Println(arg+":", err)
			continue
		}
		filtered := []User{}
		for _, user := range users {
			if user.FollowersCount >= *minFollowers {
				filtered = append(filtered, user)
			}
		}
		if *sortByFollowers {
			sort.Sort(byUserFollowersCount(filtered))
		}
		if *maxUsers > 0 && *maxUsers < len(filtered) {
			filtered = filtered[:*maxUsers]
		}
		allUsers = append(allUsers, filtered...)
	}

	if *getLiveInfo {
		getRoomInfo(allUsers)
	}

	if *showOnlyLive {
		allUsers = filterLiveUsers(allUsers)
	}

	if *useJson {
		json.NewEncoder(os.Stdout).Encode(allUsers)
	} else if *useTable {
		printTable(allUsers, *getLiveInfo)
	} else {
		for _, user := range allUsers {
			fmt.Println(user.Name)
		}
	}
}

func newContext() (context.Context, context.CancelFunc) {
	ctx := context.Background()
	if wsDebugUrl != "" {
		ctx, _ = chromedp.NewRemoteAllocator(ctx, wsDebugUrl)
	}
	return chromedp.NewContext(ctx)
}

func getResponse(query string) ([]byte, error) {
	ctx, cancel := newContext()
	defer cancel()

	pageUrl := (&url.URL{
		Scheme:   "https",
		Host:     "www.douyin.com",
		Path:     "/search/" + query,
		RawQuery: "source=normal_search&type=user",
	}).String()
	if verbosive {
		log.Println("visiting", pageUrl)
	}

	chanResponse := make(chan []byte)
	chanError := make(chan error)

	chromedp.ListenTarget(ctx, func(v interface{}) {
		ev, ok := v.(*network.EventResponseReceived)
		if !ok {
			return
		}
		if ev.Type != network.ResourceTypeXHR ||
			!strings.Contains(ev.Response.URL, "/aweme/v1/web/discover/search/") {
			return
		}
		if verbosive {
			log.Println("getting", ev.Response.URL)
		}
		go func() {
			c := chromedp.FromContext(ctx)
			rbp := network.GetResponseBody(ev.RequestID)
			body, err := rbp.Do(cdp.WithExecutor(ctx, c.Target))
			if err != nil {
				chanError <- err
				return
			}
			chanResponse <- body
		}()
	})
	go func() {
		err := chromedp.Run(
			ctx,
			network.Enable(),
			chromedp.Emulate(device.IPhoneX),
			chromedp.Navigate(pageUrl),
			chromedp.Sleep(20*time.Second),
		)
		if err != nil {
			chanError <- err
		}
	}()

	select {
	case response := <-chanResponse:
		chromedp.Cancel(ctx)
		return response, nil
	case err := <-chanError:
		return nil, err
	case <-time.After(20 * time.Second):
		return nil, errors.New("timed out")
	}
}

func getUsers(data []byte) (users []User, err error) {
	var resp response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}
	for _, user := range resp.UserList {
		var picture string
		pictures := user.UserInfo.AvatarThumb.URLList
		if len(pictures) > 0 {
			picture = pictures[0]
		}
		users = append(users, User{
			Id:             strToId(user.UserInfo.ShortID),
			UniqueId:       strToId(user.UserInfo.UID),
			SecUid:         user.UserInfo.SecUID,
			Name:           user.UserInfo.UniqueID,
			NickName:       user.UserInfo.Nickname,
			Description:    user.UserInfo.Signature,
			Picture:        picture,
			FollowersCount: user.UserInfo.FollowerCount,
			FavoritedCount: user.UserInfo.TotalFavorited,
		})
	}
	return
}

func strToId(in string) Id {
	out, _ := strconv.ParseUint(in, 10, 64)
	return Id(out)
}

func printTable(users []User, showRoom bool) {
	firstLine := []string{"ID", "NAME", "FOLLOWERS", "FAVORITED"}
	if showRoom {
		firstLine = append(firstLine, "ROOM CREATED")
	}
	firstLine = append(firstLine, "NICK NAME")
	lines := [][]string{firstLine}
	max := []int{}
	for _, c := range firstLine {
		l := utf8.RuneCountInString(c)
		max = append(max, l)
	}
	for _, user := range users {
		line := []string{
			fmt.Sprint(user.UniqueId),
			fmt.Sprint(user.Name),
			fmt.Sprint(user.FollowersCount),
			fmt.Sprint(user.FavoritedCount),
		}
		if showRoom {
			if user.Room == nil {
				line = append(line, "-")
			} else {
				duration := time.Since(user.Room.CreatedAt).Round(time.Second)
				line = append(line, duration.String())
			}
		}
		line = append(line, fmt.Sprint(user.NickName))
		for i, f := range line {
			l := utf8.RuneCountInString(f)
			if l > max[i] {
				max[i] = l
			}
		}
		lines = append(lines, line)
	}
	formats := []string{}
	for _, n := range max {
		formats = append(formats, fmt.Sprintf("%%-%ds", n))
	}
	format := strings.Join(formats, "  ") + "\n"
	for _, line := range lines {
		l := []interface{}{}
		for _, c := range line {
			l = append(l, c)
		}
		fmt.Printf(format, l...)
	}
}

func filterLiveUsers(users []User) (filtered []User) {
	for _, user := range users {
		if user.Room != nil {
			filtered = append(filtered, user)
		}
	}
	return
}

func getRoomInfo(users []User) {
	names := make(chan int)
	go func() {
		defer close(names)
		for i := range users {
			names <- i
		}
	}()
	concurrency := 4
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for i := range names {
				if verbosive {
					log.Println(users[i].Name+":", "getting live info")
				}
				user, err := douyinapi.GetUserByName(users[i].Name)
				if err != nil {
					log.Println(users[i].Name+":", err)
					continue
				}
				users[i].Room = user.Room
			}
		}()
	}
	wg.Wait()
}
