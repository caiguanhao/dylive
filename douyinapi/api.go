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
	HttpTimeout = 5 * time.Second

	Verbose = false

	ErrorNoUrl = errors.New("No URL found")

	reShareUrl = regexp.MustCompile(`https?://v\.douyin\.com/([A-Za-z0-9]{7,})/`)
	reShareId  = regexp.MustCompile(`[A-Za-z0-9]{7,}`)
	reId       = regexp.MustCompile(`[0-9]{10,20}`)
)

type (
	User struct {
		Id              uint64
		IdString        string `json:"uid"`
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
		User User `json:"user"`
	}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		if Verbose {
			log.Println("GetRoomId:", err)
		}
		return
	}
	if res.User.Name != "" {
		user = &res.User
		user.Id = userId
	}
	return
}

// Get live stream URL of a page URL.
func GetLiveUrlFromRoomId(roomId uint64) (urlMap map[string]string, err error) {
	return GetLiveUrlFromUrl("https://webcast.amemv.com/webcast/reflow/" + strconv.FormatUint(roomId, 10))
}

// Get live stream URL of a page URL.
func GetLiveUrlFromUrl(url string) (urlMap map[string]string, err error) {
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
	urlMap = GetLiveUrlFromHtml(string(b))
	return
}

// Get live stream URL in page HTML content.
func GetLiveUrlFromHtml(html string) (urlMap map[string]string) {
	i := strings.Index(html, scriptOpen)
	if i < 0 {
		return
	}
	html = html[i+len(scriptOpen):]
	i = strings.Index(html, scriptClose)
	if i < 0 {
		return
	}
	html = html[:i]
	var obj map[string]map[string]map[string]map[string]map[string]string
	json.Unmarshal([]byte(html), &obj)
	for _, v := range obj {
		var ok bool
		urlMap, ok = v["room"]["stream_url"]["hls_pull_url_map"]
		if ok {
			return
		}
	}
	return
}
