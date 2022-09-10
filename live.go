package dylive

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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
		App struct {
			LayoutData struct {
				CategoryTab struct {
					CategoryData []struct {
						Partition struct {
							IDStr string `json:"id_str"`
							Type  int    `json:"type"`
							Title string `json:"title"`
						} `json:"partition"`
					} `json:"categoryData"`
				} `json:"categoryTab"`
			} `json:"layoutData"`
		} `json:"app"`
		Four struct {
			PartitionData struct {
				Partition struct {
					IDStr string `json:"id_str"`
					Type  int    `json:"type"`
					Title string `json:"title"`
				} `json:"partition"`
				SubPartition []struct {
					IDStr string `json:"id_str"`
					Type  int    `json:"type"`
					Title string `json:"title"`
				} `json:"sub_partition"`
			} `json:"partitionData"`
		} `json:"874cbd3ca82b27af9f285883fd26e52f"`
	}
)

// GetCategories gets all Douyin live stream categories.
func GetCategories(ctx context.Context) ([]Category, error) {
	const first = "1_620"
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
	rd, err := getCategoryPageRenderData(ctx, id)
	if err != nil {
		return err
	}
	var cats dyliveCategories
	err = json.Unmarshal([]byte(rd), &cats)
	if err != nil {
		return err
	}
	if categories != nil {
		for _, cat := range cats.App.LayoutData.CategoryTab.CategoryData {
			*categories = append(*categories, Category{
				Id:   fmt.Sprintf("%d_%s", cat.Partition.Type, cat.Partition.IDStr),
				Name: cat.Partition.Title,
			})
		}
	}
	if subCategories != nil {
		p := cats.Four.PartitionData.Partition
		for _, cat := range cats.Four.PartitionData.SubPartition {
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
		StreamUrl         string
		CurrentUsersCount string
		TotalUsersCount   string
		Category          Category
		User              User
	}

	User struct {
		Name    string
		Picture string
	}

	dyliveCategory struct {
		Four struct {
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
			} `json:"partitionData"`
		} `json:"874cbd3ca82b27af9f285883fd26e52f"`
	}
)

// GetRoomsByCategory gets top 15 Douyin live stream rooms of a category.
func GetRoomsByCategory(ctx context.Context, categoryId string) ([]Room, error) {
	rd, err := getCategoryPageRenderData(ctx, categoryId)
	if err != nil {
		return nil, err
	}
	var cat dyliveCategory
	err = json.Unmarshal([]byte(rd), &cat)
	if err != nil {
		return nil, err
	}
	var rooms []Room
	for _, room := range cat.Four.RoomsData.Data {
		p := cat.Four.PartitionData.Partition
		c := cat.Four.PartitionData.SelectPartition
		rooms = append(rooms, Room{
			Name:              room.Room.Title,
			CoverUrl:          room.Cover,
			WebUrl:            "https://live.douyin.com/" + room.WebRid,
			StreamUrl:         room.StreamSrc,
			CurrentUsersCount: room.Room.Stats.UserCountStr,
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

func getCategoryPageRenderData(ctx context.Context, id string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://live.douyin.com/category/"+id, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return getRenderData(string(b)), nil
}

func getRenderData(input string) (output string) {
	a := strings.Index(input, "RENDER_DATA")
	if a < 0 {
		return
	}
	input = input[a:]
	a = strings.Index(input, ">")
	if a < 0 {
		return
	}
	input = input[a+1:]
	a = strings.Index(input, "<")
	if a < 0 {
		return
	}
	input = input[:a]
	input, err := url.QueryUnescape(input)
	if err != nil {
		return
	}
	output = input
	return
}
