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

const userAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1"

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
	const first = "1_4609"
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

type (
	Room struct {
		Name              string
		CoverUrl          string
		WebUrl            string
		CurrentUsersCount string
		TotalUsersCount   string
		Category          Category
		User              User
		StreamUrl         string
		FlvStreamUrls     map[string]string
		HlsStreamUrls     map[string]string
	}

	User struct {
		Name    string
		Picture string
	}

	dyliveCategory struct {
		RoomsData struct {
			Data []struct {
				Room struct {
					Title string `json:"title"`
					Stats struct {
						TotalUserStr string `json:"total_user_str"`
						UserCountStr string `json:"user_count_str"`
					} `json:"stats"`
					Owner struct {
						Nickname string `json:"nickname"`
					} `json:"owner"`
					StreamUrl struct {
						FlvPullUrl    map[string]string `json:"flv_pull_url"`
						HlsPullUrlMap map[string]string `json:"hls_pull_url_map"`
					} `json:"stream_url"`
					RoomViewStats struct {
						DisplayValue int `json:"display_value"`
					} `json:"room_view_stats"`
				} `json:"room"`
				WebRid    string `json:"web_rid"`
				StreamSrc string `json:"streamSrc"`
				Cover     string `json:"cover"`
				Avatar    string `json:"avatar"`
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
			Name:              room.Room.Title,
			CoverUrl:          room.Cover,
			WebUrl:            "https://live.douyin.com/" + room.WebRid,
			StreamUrl:         room.StreamSrc,
			FlvStreamUrls:     room.Room.StreamUrl.FlvPullUrl,
			HlsStreamUrls:     room.Room.StreamUrl.HlsPullUrlMap,
			CurrentUsersCount: count,
			TotalUsersCount:   room.Room.Stats.TotalUserStr,
			Category: Category{
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
