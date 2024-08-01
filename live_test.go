package dylive

import (
	"context"
	"encoding/json"
	"os"
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
	for _, cat := range categories {
		if len(cat.Categories) < 1 {
			t.Error("sub category should not be empty")
		}
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
	rooms, err := GetRoomsByCategory(ctx, "1_2_1_1010045")
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
