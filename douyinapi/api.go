package douyinapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// If you don't have device ID, use this. It will become unusable after
	// a while and can be updated frequently. You can enumerate to find a
	// new one, starting from this number.
	DefaultDeviceId uint64 = 66178590526

	// Timeout for all HTTP requests.
	HttpTimeout = 5 * time.Second

	// Print URL or error.
	Verbose = false

	ErrorNoUrl  = errors.New("No URL found")
	ErrorNoRoom = errors.New("No room found")

	reShareUrl = regexp.MustCompile(`https?://v\.douyin\.com/([A-Za-z0-9]{7,})/`)
	reShareId  = regexp.MustCompile(`[A-Za-z0-9]{7,}`)
	reId       = regexp.MustCompile(`[0-9]{10,20}`)
)

type (
	User struct {
		Id              uint64
		IdString        string
		SecUid          string
		PageUrl         string
		Country         string
		Province        string
		City            string
		District        string
		Location        string
		Name            string
		Description     string
		VideosCount     int
		FollowersCount  int
		FollowingsCount int
		FavoritesCount  int
		LikesCount      int
		RoomId          uint64
		RoomIdString    string
		PictureSmall    string
		PictureMedium   string
		PictureLarge    string
	}

	Room struct {
		User *User

		Id        uint64
		IdString  string
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

		StreamId        uint64
		StreamIdString  string
		StreamHeight    int
		StreamWidth     int
		StreamHlsUrlMap map[string]string
	}

	dyUser struct {
		IdString        string `json:"uid"`
		SecUid          string `json:"sec_uid"`
		Country         string `json:"country"`
		Province        string `json:"province"`
		City            string `json:"city"`
		District        string `json:"district"`
		Location        string `json:"location"`
		Name            string `json:"nickname"`
		Description     string `json:"signature"`
		VideosCount     int    `json:"aweme_count"`
		FollowersCount  int    `json:"follower_count"`
		FollowingsCount int    `json:"following_count"`
		FavoritesCount  int    `json:"favoriting_count"`
		LikesCount      int    `json:"total_favorited"`
		RoomId          uint64 `json:"room_id"`
		PictureSmall    struct {
			List []string `json:"url_list"`
		} `json:"avatar_thumb"`
		PictureMedium struct {
			List []string `json:"url_list"`
		} `json:"avatar_medium"`
		PictureLarge struct {
			List []string `json:"url_list"`
		} `json:"avatar_larger"`
	}

	dyRoom struct {
		Room struct {
			IdString string `json:"id_str"`
			User     struct {
				IdString    string `json:"id_str"`
				Name        string `json:"nickname"`
				Description string `json:"signature"`
				FollowInfo  struct {
					FollowersCount  int `json:"follower_count"`
					FollowingsCount int `json:"following_count"`
				} `json:"follow_info"`
				PictureSmall struct {
					List []string `json:"url_list"`
				} `json:"avatar_thumb"`
				PictureMedium struct {
					List []string `json:"url_list"`
				} `json:"avatar_medium"`
				PictureLarge struct {
					List []string `json:"url_list"`
				} `json:"avatar_large"`
			} `json:"owner"`
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
		} `json:"room"`
	}
)

const (
	scriptOpen  = "<script>window.__INIT_PROPS__ = "
	scriptClose = "</script>"
)

// Get page URL in a share message. Empty is returned if no URL is found.
func GetPageUrl(input string) string {
	url, _ := GetPageUrlStr(input)
	return url
}

// Just like GetPageUrl, but also returns the matching string.
func GetPageUrlStr(input string) (string, string) {
	if url := reShareUrl.FindString(input); url != "" {
		return url, url
	}
	if id := reShareId.FindString(input); id != "" {
		return "https://v.douyin.com/" + id + "/", id
	}
	return "", ""
}

// Get user ID or live stream ID (room ID) from page URL.
func GetIdFromUrl(url string) (userId, roomId uint64, err error) {
	if strings.HasPrefix(url, "https://www.iesdouyin.com/share/user/") {
		if id := reId.FindString(url); id != "" {
			userId, err = strconv.ParseUint(id, 10, 64)
		}
		return
	}
	if strings.HasPrefix(url, "https://webcast.amemv.com/webcast/reflow/") {
		if id := reId.FindString(url); id != "" {
			roomId, err = strconv.ParseUint(id, 10, 64)
		}
		return
	}
	if strings.HasPrefix(url, "https://v.douyin.com/") {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: HttpTimeout,
		}
		var resp *http.Response
		resp, err = client.Get(GetPageUrl(url))
		if err != nil {
			return
		}
		url = resp.Header.Get("Location")
		if !strings.HasPrefix(url, "https://v.douyin.com/") {
			return GetIdFromUrl(url)
		}
	}
	return
}

// Get user info. The server may reply empty user info, so user can be nil even
// when error is nil.
func GetUserInfo(deviceId, userId uint64) (user *User, err error) {
	user, err = getUserInfo(deviceId, userId)
	if user == nil { // try again
		user, err = getUserInfo(deviceId, userId)
	}
	return
}

func getUserInfo(deviceId, userId uint64) (user *User, err error) {
	url := "https://api3-core-c-lf.amemv.com/aweme/v1/user/profile/other/"
	url += fmt.Sprintf("?device_id=%d&aid=1128&user_id=%d", deviceId, userId)
	if Verbose {
		log.Println("GetRoomId:", url)
	}
	client := &http.Client{
		Timeout: HttpTimeout,
	}
	var resp *http.Response
	resp, err = client.Get(url)
	if err != nil {
		if Verbose {
			log.Println("GetRoomId:", err)
		}
		return
	}
	defer resp.Body.Close()
	var res struct {
		User dyUser `json:"user"`
	}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		if Verbose {
			log.Println("GetRoomId:", err)
		}
		return
	}
	if res.User.Name != "" {
		u := res.User
		user = &User{
			Id:              userId,
			IdString:        u.IdString,
			SecUid:          u.SecUid,
			PageUrl:         userUrl(userId, u.SecUid),
			Country:         u.Country,
			Province:        u.Province,
			City:            u.City,
			District:        u.District,
			Location:        u.Location,
			Name:            u.Name,
			Description:     u.Description,
			VideosCount:     u.VideosCount,
			FollowersCount:  u.FollowersCount,
			FollowingsCount: u.FollowingsCount,
			FavoritesCount:  u.FavoritesCount,
			LikesCount:      u.LikesCount,
			RoomId:          u.RoomId,
			RoomIdString:    strconv.FormatUint(u.RoomId, 10),
			PictureSmall:    getFirst(u.PictureSmall.List),
			PictureMedium:   getFirst(u.PictureMedium.List),
			PictureLarge:    getFirst(u.PictureLarge.List),
		}
	}
	return
}

// Get room by ID.
func GetRoom(id uint64) (*Room, error) {
	return GetRoomFromUrl(roomUrl(id))
}

// Get room info from a page URL.
func GetRoomFromUrl(url string) (room *Room, err error) {
	if url == "" {
		err = ErrorNoUrl
		return
	}
	var req *http.Request
	req, err = http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1")
	if err != nil {
		return
	}
	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var b []byte
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	room = GetRoomFromHtml(string(b))
	if room == nil {
		err = ErrorNoRoom
	}
	return
}

// Get room info from a page HTML content.
func GetRoomFromHtml(html string) *Room {
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

	var obj map[string]dyRoom
	json.Unmarshal(bytes, &obj)
	for _, o := range obj {
		if o.Room.IdString == "" {
			continue
		}
		return &Room{
			User: &User{
				Id:              strToUint64(o.Room.User.IdString),
				IdString:        o.Room.User.IdString,
				Name:            o.Room.User.Name,
				Description:     o.Room.User.Description,
				FollowersCount:  o.Room.User.FollowInfo.FollowersCount,
				FollowingsCount: o.Room.User.FollowInfo.FollowingsCount,
				RoomId:          strToUint64(o.Room.IdString),
				RoomIdString:    o.Room.IdString,
				PictureSmall:    getFirst(o.Room.User.PictureSmall.List),
				PictureMedium:   getFirst(o.Room.User.PictureMedium.List),
				PictureLarge:    getFirst(o.Room.User.PictureLarge.List),
			},
			Id:                      strToUint64(o.Room.IdString),
			IdString:                o.Room.IdString,
			PageUrl:                 roomUrl(strToUint64(o.Room.IdString)),
			Title:                   o.Room.Title,
			Status:                  o.Room.Status,
			Operating:               o.Room.Status == 2,
			CreatedAt:               time.Unix(int64(o.Room.CreatedAt), 0).UTC(),
			LikesCount:              o.Room.LikesCount,
			CurrentUsersCount:       o.Room.CurrentUsersCount,
			NewFollowersCount:       o.Room.Stats.NewFollowersCount,
			GiftsUniqueVisitorCount: o.Room.Stats.GiftsUniqueVisitorCount,
			FansCount:               o.Room.Stats.FansCount,
			TotalUsersCount:         o.Room.Stats.TotalUsersCount,
			StreamId:                strToUint64(o.Room.StreamUrl.IdString),
			StreamIdString:          o.Room.StreamUrl.IdString,
			StreamHeight:            o.Room.StreamUrl.Extra.Height,
			StreamWidth:             o.Room.StreamUrl.Extra.Width,
			StreamHlsUrlMap:         o.Room.StreamUrl.HlsUrlMap,
		}
	}

	// alternative way if json parsing error
	var obj2 map[string]map[string]map[string]map[string]map[string]string
	json.Unmarshal(bytes, &obj2)
	for _, v := range obj2 {
		urlMap, ok := v["room"]["stream_url"]["hls_pull_url_map"]
		if !ok {
			continue
		}
		return &Room{
			StreamHlsUrlMap: urlMap,
		}
	}

	return nil
}

func userUrl(id uint64, secUid string) string {
	return "https://www.iesdouyin.com/share/user/" + strconv.FormatUint(id, 10) + "?sec_uid=" + secUid
}

func roomUrl(id uint64) string {
	return "https://webcast.amemv.com/webcast/reflow/" + strconv.FormatUint(id, 10)
}

func getFirst(input []string) (out string) {
	if len(input) > 0 {
		out = input[0]
	}
	return
}

func strToUint64(in string) (out uint64) {
	out, _ = strconv.ParseUint(in, 10, 64)
	return
}
