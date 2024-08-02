package dylive

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/115.0"

type (
	Category struct {
		Id         string
		Name       string
		Categories []Category
	}

	dyliveCategories struct {
		CategoryData []struct {
			Partition struct {
				IDStr string `json:"id_str"`
				Type  int    `json:"type"`
				Title string `json:"title"`
			} `json:"partition"`
		} `json:"categoryData"`
	}
)

// GetCategories gets all Douyin live stream categories.
func GetCategories(ctx context.Context) ([]Category, error) {
	const first = "1_1"
	var cats []Category
	var subCats []Category
	err := getCategories(ctx, first, &cats, &subCats)
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	for i := range cats {
		if cats[i].Id == first {
			cats[i].Categories = subCats
		} else {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				var subCats []Category
				getCategories(ctx, cats[i].Id, nil, &subCats)
				cats[i].Categories = subCats
			}(i)
		}
	}
	wg.Wait()
	return cats, nil
}

func getCategories(ctx context.Context, id string, categories, subCategories *[]Category) error {
	if categories == nil && subCategories == nil {
		return nil
	}
	data, err := getCategoryPageData(ctx, id, "categoryData", "partitionData")
	if err != nil {
		return err
	}
	categoryData, partitionData := data[0], data[1]
	var cats dyliveCategories
	if err := getDataInArray(categoryData, &cats); err != nil {
		return err
	}
	var cat dyliveCategory
	if err := getDataInArray(partitionData, &cat); err != nil {
		return err
	}
	if categories != nil {
		for _, cat := range cats.CategoryData {
			*categories = append(*categories, Category{
				Id:   fmt.Sprintf("%d_%s", cat.Partition.Type, cat.Partition.IDStr),
				Name: cat.Partition.Title,
			})
		}
	}
	if subCategories != nil {
		p := cat.PartitionData.Partition
		for _, cat := range cat.PartitionData.SubPartition {
			*subCategories = append(*subCategories, Category{
				Id:   fmt.Sprintf("%d_%s_%d_%s", p.Type, p.IDStr, cat.Type, cat.IDStr),
				Name: cat.Title,
			})
		}
	}
	return nil
}

const (
	RoomStatusLiveOn RoomStatus = 2 + iota
	_
	RoomStatusLiveOff
)

type (
	RoomStatus = int

	Room struct {
		Id                string
		DouyinId          string
		StatusCode        RoomStatus
		Name              string
		CoverUrl          string
		WebUrl            string
		CurrentUsersCount string
		TotalUsersCount   string
		Category          *Category
		User              User
		StreamUrl         string
		FlvStreamUrls     map[string]string
		HlsStreamUrls     map[string]string
	}

	User struct {
		Name    string
		Picture string
	}

	dyUser struct {
		Nickname    string `json:"nickname"`
		AvatarThumb struct {
			UrlList []string `json:"url_list"`
		} `json:"avatar_thumb"`
	}

	dyliveRoom struct {
		IdStr  string `json:"id_str"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Cover  struct {
			UrlList []string `json:"url_list"`
		} `json:"cover"`
		Stats struct {
			TotalUserStr string `json:"total_user_str"`
			UserCountStr string `json:"user_count_str"`
		} `json:"stats"`
		Owner     dyUser `json:"owner"`
		StreamUrl struct {
			FlvPullUrl        map[string]string `json:"flv_pull_url"`
			HlsPullUrlMap     map[string]string `json:"hls_pull_url_map"`
			DefaultResolution string            `json:"default_resolution"`
		} `json:"stream_url"`
		RoomViewStats struct {
			DisplayValue int `json:"display_value"`
		} `json:"room_view_stats"`
	}

	dyliveCategory struct {
		RoomsData struct {
			Data []struct {
				Room      dyliveRoom `json:"room"`
				WebRid    string     `json:"web_rid"`
				StreamSrc string     `json:"streamSrc"`
				Cover     string     `json:"cover"`
				Avatar    string     `json:"avatar"`
			} `json:"data"`
		} `json:"roomsData"`
		PartitionData struct {
			Partition struct {
				IDStr string `json:"id_str"`
				Type  int    `json:"type"`
				Title string `json:"title"`
			} `json:"partition"`
			SelectPartition struct {
				IDStr string `json:"id_str"`
				Type  int    `json:"type"`
				Title string `json:"title"`
			} `json:"select_partition"`
			SubPartition []struct {
				IDStr string `json:"id_str"`
				Type  int    `json:"type"`
				Title string `json:"title"`
			} `json:"sub_partition"`
		} `json:"partitionData"`
	}
)

// FlvUrlForQuality returns the .flv stream URL for the given quality (uhd, hd, ld, sd).
// If no matching URL is found, it returns the room's default StreamUrl.
func (room Room) FlvUrlForQuality(quality string) string {
	return room.urlForQuality(room.FlvStreamUrls, quality)
}

// HlsUrlForQuality returns the .m3u8 stream URL for the given quality (uhd, hd, ld, sd).
// If no matching URL is found, it returns the room's default StreamUrl.
func (room Room) HlsUrlForQuality(quality string) string {
	return room.urlForQuality(room.HlsStreamUrls, quality)
}

func (room Room) urlForQuality(urls map[string]string, quality string) string {
	quality = strings.ToLower(quality)
	for key, value := range urls {
		switch quality {
		case "uhd":
			if strings.Contains(key, "FULL_HD") || strings.Contains(value, "_uhd") {
				return value
			}
		case "hd":
			if strings.Contains(value, "_hd") {
				return value
			}
		case "ld":
			if strings.Contains(value, "_ld") {
				return value
			}
		case "sd":
			if strings.Contains(value, "_sd") {
				return value
			}
		default:
			return room.StreamUrl
		}
	}
	return room.StreamUrl
}

// GetRoomsByCategory gets top 15 Douyin live stream rooms of a category.
func GetRoomsByCategory(ctx context.Context, categoryId string) ([]Room, error) {
	data, err := getCategoryPageData(ctx, categoryId, "roomsData")
	if err != nil {
		return nil, err
	}
	roomsData := data[0]
	var cat dyliveCategory
	if err := getDataInArray(roomsData, &cat); err != nil {
		return nil, err
	}

	var rooms []Room
	for _, room := range cat.RoomsData.Data {
		p := cat.PartitionData.Partition
		c := cat.PartitionData.SelectPartition
		var count string
		if room.Room.RoomViewStats.DisplayValue > 0 {
			count = strconv.Itoa(room.Room.RoomViewStats.DisplayValue)
		} else {
			count = room.Room.Stats.UserCountStr
		}
		rooms = append(rooms, Room{
			Id:                room.Room.IdStr,
			DouyinId:          room.WebRid,
			StatusCode:        RoomStatusLiveOn,
			Name:              room.Room.Title,
			CoverUrl:          room.Cover,
			WebUrl:            "https://live.douyin.com/" + room.WebRid,
			StreamUrl:         room.StreamSrc,
			FlvStreamUrls:     room.Room.StreamUrl.FlvPullUrl,
			HlsStreamUrls:     room.Room.StreamUrl.HlsPullUrlMap,
			CurrentUsersCount: count,
			TotalUsersCount:   room.Room.Stats.TotalUserStr,
			Category: &Category{
				Id:   fmt.Sprintf("%d_%s", p.Type, p.IDStr),
				Name: p.Title,
				Categories: []Category{
					{
						Id:   fmt.Sprintf("%d_%s_%d_%s", p.Type, p.IDStr, c.Type, c.IDStr),
						Name: c.Title,
					},
				},
			},
			User: User{
				Name:    room.Room.Owner.Nickname,
				Picture: room.Avatar,
			},
		})
	}
	return rooms, nil
}

func getCategoryPageData(ctx context.Context, id string, filters ...string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://live.douyin.com/category/"+id, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	parts := getDataInHtml(string(b))
	var output []string
	for _, filter := range filters {
		var ret string
		for _, part := range parts {
			if strings.Contains(part, filter) {
				ret = part
				break
			}
		}
		output = append(output, ret)
	}
	return output, nil
}

type (
	dyliveRoomDetails struct {
		State struct {
			RoomStore struct {
				RoomInfo struct {
					Room   dyliveRoom `json:"room"`
					WebRid string     `json:"web_rid"`
					Anchor dyUser     `json:"anchor"`
				} `json:"roomInfo"`
			} `json:"roomStore"`
		} `json:"state"`
	}
)

// GetRoom get live stream room details by Douyin ID (抖音号)
func GetRoom(ctx context.Context, douyinId string) (*Room, error) {
	data, err := getLivePageData(ctx, douyinId, "roomId")
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || data[0] == "" {
		return nil, fmt.Errorf("DouyinId %s does not exist", douyinId)
	}
	roomsData := data[0]
	var page dyliveRoomDetails
	if err := getDataInArray(roomsData, &page); err != nil {
		return nil, err
	}

	info := page.State.RoomStore.RoomInfo

	var cover string
	if len(info.Room.Cover.UrlList) > 0 {
		cover = info.Room.Cover.UrlList[0]
	}

	streamUrl := info.Room.StreamUrl.FlvPullUrl[info.Room.StreamUrl.DefaultResolution]

	var count string
	if info.Room.RoomViewStats.DisplayValue > 0 {
		count = strconv.Itoa(info.Room.RoomViewStats.DisplayValue)
	} else {
		count = info.Room.Stats.UserCountStr
	}

	userName := info.Room.Owner.Nickname
	if userName == "" {
		userName = info.Anchor.Nickname
	}

	var userPicture string
	if len(info.Room.Owner.AvatarThumb.UrlList) > 0 {
		userPicture = info.Room.Owner.AvatarThumb.UrlList[0]
	} else if len(info.Anchor.AvatarThumb.UrlList) > 0 {
		userPicture = info.Anchor.AvatarThumb.UrlList[0]
	}

	return &Room{
		Id:                info.Room.IdStr,
		DouyinId:          info.WebRid,
		StatusCode:        info.Room.Status,
		Name:              info.Room.Title,
		CoverUrl:          cover,
		WebUrl:            "https://live.douyin.com/" + info.WebRid,
		StreamUrl:         streamUrl,
		FlvStreamUrls:     info.Room.StreamUrl.FlvPullUrl,
		HlsStreamUrls:     info.Room.StreamUrl.HlsPullUrlMap,
		CurrentUsersCount: count,
		TotalUsersCount:   info.Room.Stats.TotalUserStr,
		User: User{
			Name:    userName,
			Picture: userPicture,
		},
	}, nil
}

func getLivePageData(ctx context.Context, douyinId string, filters ...string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://live.douyin.com/"+douyinId, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", "__ac_nonce=064caded4009deafd8b89")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	parts := getDataInHtml(string(b))
	var output []string
	for _, filter := range filters {
		var ret string
		for _, part := range parts {
			if strings.Contains(part, filter) {
				ret = part
				break
			}
		}
		output = append(output, ret)
	}
	return output, nil
}

func getDataInHtml(input string) (output []string) {
	const funcName = "__pace_f"
	const endTag = "</script>"
	var parts []string
	for {
		a := strings.Index(input, funcName)
		if a == -1 {
			break
		}
		input = input[a+len(funcName):]
		b := strings.Index(input, `"`)
		if b < 0 {
			continue
		}
		input = input[b+1:]
		b = strings.Index(input, endTag)
		if b < 0 {
			continue
		}
		b = strings.LastIndex(input[:b], `"`)
		if b < 0 {
			continue
		}
		var ret string
		if json.Unmarshal([]byte(`"`+input[:b]+`"`), &ret) != nil {
			continue
		}
		parts = append(parts, ret)
	}
	parts = strings.Split(strings.Join(parts, "\n"), "\n")
	for _, part := range parts {
		a := strings.IndexAny(part, "[{")
		if a == -1 {
			continue
		}
		b := strings.LastIndexAny(part, "}]")
		if b == -1 {
			continue
		}
		output = append(output, part[a:b+1])
	}
	return
}

func getDataInArray(input string, target interface{}) error {
	var array []interface{}
	if err := json.Unmarshal([]byte(input), &array); err != nil {
		return err
	}
	for _, element := range array {
		switch v := element.(type) {
		case map[string]interface{}:
			jsonStr, err := json.Marshal(v)
			if err != nil {
				continue
			}
			return json.Unmarshal(jsonStr, target)
		}
	}
	return nil
}
