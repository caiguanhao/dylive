package douyinapi

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	// Timeout for all HTTP requests
	HttpTimeout = 5 * time.Second

	// User-Agent header for all HTTP requests
	UserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1"

	ErrInvalidData = errors.New("invalid page data")
	ErrNoSuchUser  = errors.New("no such user")
	ErrorNoUrl     = errors.New("No URL found")
	ErrorNoRoom    = errors.New("No room found")
)

type (
	Id uint64

	User struct {
		Id              Id     // will change over time
		UniqueId        Id     // will change over time
		SecUid          string // constant
		Name            string // user id in user profile page
		NickName        string // aka "display name"
		Picture         string // url of thumbnail picture
		FollowersCount  int    // followers count
		FollowingsCount int    // followings count
		Room            *Room  // live stream room (if any)
	}

	Room struct {
		Id        Id
		PageUrl   string
		Title     string
		Status    int
		Operating bool
		CreatedAt time.Time

		LikesCount              int
		CurrentUsersCount       int
		NewFollowersCount       int
		GiftsUniqueVisitorCount int
		FansCount               int
		TotalUsersCount         int

		StreamId        Id
		StreamHeight    int
		StreamWidth     int
		StreamHlsUrlMap map[string]string
	}
)

func (id Id) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(id), 10))
}

func GetUserByName(name string) (user *User, err error) {
	url := "https://live.douyin.com/" + name
	var req *http.Request
	req, err = http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", UserAgent)
	if err != nil {
		return
	}
	var resp *http.Response
	client := &http.Client{
		Timeout: HttpTimeout,
	}
	resp, err = client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var b []byte
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	return parseLivePageHtml(string(b))
}

type (
	livePageData struct {
		Location string `json:"location"`
		Odin     struct {
			UserID       string `json:"user_id"`
			UserUniqueID string `json:"user_unique_id"`
		} `json:"odin"`
		RouteInitialProps struct {
			ErrorType string `json:"errorType"`
			RoomInfo  struct {
				Room   *dyRoom `json:"room"`
				RoomID string  `json:"roomId"`
				Anchor struct {
					Nickname    string `json:"nickname"`
					AvatarThumb struct {
						URLList []string `json:"url_list"`
					} `json:"avatar_thumb"`
					FollowInfo struct {
						FollowingCount int `json:"following_count"`
						FollowerCount  int `json:"follower_count"`
					} `json:"follow_info"`
					SecUID string `json:"sec_uid"`
				} `json:"anchor"`
			} `json:"roomInfo"`
		} `json:"routeInitialProps"`
	}

	dyRoom struct {
		IdString          string `json:"id_str"`
		Title             string `json:"title"`
		LikesCount        int    `json:"like_count"`
		CurrentUsersCount int    `json:"user_count"`
		Status            int    `json:"status"` // 2 - started, 4 - ended
		CreatedAt         int    `json:"create_time"`
		Stats             struct {
			NewFollowersCount       int `json:"follow_count"`
			GiftsUniqueVisitorCount int `json:"gift_uv_count"` // maybe
			FansCount               int `json:"fan_ticket"`    // maybe
			TotalUsersCount         int `json:"total_user"`
		} `json:"stats"`
		StreamUrl struct {
			IdString string `json:"id_str"`
			Extra    struct {
				Height int `json:"height"`
				Width  int `json:"width"`
			} `json:"extra"`
			HlsUrlMap map[string]string `json:"hls_pull_url_map"`
		} `json:"stream_url"`
	}
)

func (room *dyRoom) toRoom() *Room {
	if room == nil {
		return nil
	}
	return &Room{
		Id:                      strToId(room.IdString),
		PageUrl:                 roomUrl(room.IdString),
		Title:                   room.Title,
		Status:                  room.Status,
		Operating:               room.Status == 2,
		CreatedAt:               time.Unix(int64(room.CreatedAt), 0).UTC(),
		LikesCount:              room.LikesCount,
		CurrentUsersCount:       room.CurrentUsersCount,
		NewFollowersCount:       room.Stats.NewFollowersCount,
		GiftsUniqueVisitorCount: room.Stats.GiftsUniqueVisitorCount,
		FansCount:               room.Stats.FansCount,
		TotalUsersCount:         room.Stats.TotalUsersCount,
		StreamId:                strToId(room.StreamUrl.IdString),
		StreamHeight:            room.StreamUrl.Extra.Height,
		StreamWidth:             room.StreamUrl.Extra.Width,
		StreamHlsUrlMap:         room.StreamUrl.HlsUrlMap,
	}
}

func parseLivePageHtml(html string) (*User, error) {
	a := strings.Index(html, "RENDER_DATA")
	if a < 0 {
		return nil, ErrInvalidData
	}
	html = html[a:]
	a = strings.Index(html, ">")
	if a < 0 {
		return nil, ErrInvalidData
	}
	html = html[a+1:]
	a = strings.Index(html, "<")
	if a < 0 {
		return nil, ErrInvalidData
	}
	html = html[:a]
	html, err := url.QueryUnescape(html)
	if err != nil {
		return nil, err
	}
	var data livePageData
	err = json.Unmarshal([]byte(html), &data)
	if err != nil {
		return nil, err
	}
	if data.RouteInitialProps.ErrorType == "server-error" {
		return nil, ErrNoSuchUser
	}
	var picture string
	pictures := data.RouteInitialProps.RoomInfo.Anchor.AvatarThumb.URLList
	if len(pictures) > 0 {
		picture = pictures[0]
	}
	return &User{
		Id:              strToId(data.Odin.UserID),
		UniqueId:        strToId(data.Odin.UserUniqueID),
		SecUid:          data.RouteInitialProps.RoomInfo.Anchor.SecUID,
		Room:            data.RouteInitialProps.RoomInfo.Room.toRoom(),
		Name:            strings.Trim(data.Location, "/"),
		NickName:        data.RouteInitialProps.RoomInfo.Anchor.Nickname,
		Picture:         picture,
		FollowersCount:  data.RouteInitialProps.RoomInfo.Anchor.FollowInfo.FollowerCount,
		FollowingsCount: data.RouteInitialProps.RoomInfo.Anchor.FollowInfo.FollowingCount,
	}, nil
}

func roomUrl(id string) string {
	return "https://webcast.amemv.com/webcast/reflow/" + id
}

func strToId(in string) Id {
	out, _ := strconv.ParseUint(in, 10, 64)
	return Id(out)
}

const (
	scriptOpen  = "<script>window.__INIT_PROPS__ = "
	scriptClose = "</script>"
)

func GetRoom(url string) (room *Room, err error) {
	if url == "" {
		err = ErrorNoUrl
		return
	}
	var req *http.Request
	req, err = http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", UserAgent)
	if err != nil {
		return
	}
	var resp *http.Response
	client := &http.Client{
		Timeout: HttpTimeout,
	}
	resp, err = client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var b []byte
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	room = getRoomFromHtml(string(b))
	if room == nil {
		err = ErrorNoRoom
	}
	return
}

func getRoomFromHtml(html string) *Room {
	i := strings.Index(html, scriptOpen)
	if i < 0 {
		return nil
	}
	html = html[i+len(scriptOpen):]
	i = strings.Index(html, scriptClose)
	if i < 0 {
		return nil
	}
	html = html[:i]
	bytes := []byte(html)
	var obj map[string]map[string]dyRoom
	json.Unmarshal(bytes, &obj)
	room := obj["/webcast/reflow/:id"]["room"]
	if room.IdString != "" {
		return room.toRoom()
	}
	return nil
}
