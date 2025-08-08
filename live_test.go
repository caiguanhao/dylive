package dylive

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"
)

func TestGetCategories(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	categories, err := GetCategories(ctx)
	if err != nil {
		t.Error(err)
	}
	if len(categories) < 1 {
		t.Error("categories should not be empty")
	}
	sort.Slice(categories, func(i, j int) bool {
		if categories[i].Name == "游戏" {
			return true
		}
		if categories[j].Name == "游戏" {
			return false
		}
		return i < j
	})
	if len(categories[0].Categories) < 1 {
		t.Error("sub category should not be empty")
	}
	if len(categories) > 0 {
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		e.SetEscapeHTML(false)
		e.Encode(categories[0:1])
	}
}

func TestGetRoomsByCategory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rooms, err := GetRoomsByCategory(ctx, "4_103_1_2_1_1010102")
	if err != nil {
		t.Error(err)
	}
	if len(rooms) < 1 {
		t.Error("rooms should not be empty")
	}
	if len(rooms) > 0 {
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		e.SetEscapeHTML(false)
		e.Encode(rooms[0:1])
	}
}

func TestGetRoom(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	room, err := GetRoom(ctx, "maidanglaodo")
	if err != nil {
		t.Error(err)
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	e.Encode(room)
}
