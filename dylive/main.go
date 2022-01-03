package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/caiguanhao/dylive"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	app  *tview.Application
	grid *tview.Grid

	paneCats    *tview.TextView
	paneSubCats *tview.List
	paneRooms   *tview.Table
	paneStatus  *tview.TextView

	paneSubCatsLoading *verticalText
	paneRoomsLoading   *verticalText

	categories []dylive.Category
	rooms      []dylive.Room

	lastEnterWithAlt bool

	videoPlayer string
)

const (
	title     = "dylive"
	extraKeys = `!@#$%^&*()-=[]\;',./_+{}|:"<>?`
)

func main() {
	videoPlayer = findVideoPlayer()

	app = tview.NewApplication()

	app.SetInputCapture(onKeyPressed)

	paneCats = tview.NewTextView().
		SetText(title).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false).
		SetHighlightedFunc(func(added, removed, remaining []string) {
			idx, _ := strconv.Atoi(added[0])
			idx = idx - 1
			if idx > -1 && idx < len(categories) {
				selectCategory(&categories[idx])
			}
		})

	paneStatus = tview.NewTextView()
	paneStatus.SetBorderPadding(0, 0, 1, 1)

	paneHelp := tview.NewTextView().
		SetTextAlign(tview.AlignRight).
		SetDynamicColors(true)
	paneHelp.SetBorderPadding(0, 0, 1, 1)
	paneHelp.SetText(getHelp())

	paneFooter := tview.NewFlex().
		AddItem(paneStatus, 0, 1, false).
		AddItem(paneHelp, 0, 1, false)

	paneRooms = tview.NewTable().
		SetSelectable(true, false).
		SetBorders(false).
		SetSelectedFunc(func(row, column int) {
			selectRoom(row)
		})

	paneSubCatsLoading = &verticalText{
		tview.NewTextView().SetText("正在载入…").SetTextAlign(tview.AlignCenter),
	}
	paneRoomsLoading = &verticalText{
		tview.NewTextView().SetText("正在载入…").SetTextAlign(tview.AlignCenter),
	}

	grid = tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(30, 0).
		SetBorders(true).
		AddItem(paneCats,
			0, 0, // row, column
			1, 2, // rowSpan, colSpan
			0, 0, // minGridHeight, minGridWidth
			false). // focus
		AddItem(paneRoomsLoading,
			1, 1, // row, column
			1, 1, // rowSpan, colSpan
			0, 0, // minGridHeight, minGridWidth
			false). // focus
		AddItem(paneFooter,
			2, 0, // row, column
			1, 2, // rowSpan, colSpan
			0, 0, // minGridHeight, minGridWidth
			false) // focus

	selectCategory(nil)

	go getCategories()

	if err := app.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func getCurrentCategoryNumber() int {
	hl := paneCats.GetHighlights()
	if len(hl) < 1 {
		return 1
	}
	index, _ := strconv.Atoi(hl[0])
	if index < 1 {
		return 1
	}
	return index
}

func getCategories() {
	paneStatus.SetText("正在获取分类…")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	categories, err = dylive.GetCategories(ctx)
	if err != nil {
		paneStatus.SetText(err.Error())
		return
	}
	paneStatus.SetText("成功获取分类")
	paneCats.Clear()
	for i, cat := range categories {
		if i > 0 {
			fmt.Fprintf(paneCats, "  ")
		}
		fmt.Fprintf(paneCats, `%d ["%d"][darkcyan]%s[white][""]`, i+1, i+1, cat.Name)
	}
	paneCats.Highlight("1")
	app.Draw()
}

func getRooms(id, name string) {
	paneStatus.SetText(fmt.Sprintf("正在获取「%s」的直播列表…", name))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	rooms, err = dylive.GetRoomsByCategory(ctx, id)
	if err != nil {
		paneStatus.SetText(err.Error())
		return
	}
	paneStatus.SetText(fmt.Sprintf("成功获取「%s」的直播列表", name))
	paneRooms.Clear()
	paneRooms.Select(0, 0)
	for i, room := range rooms {
		var key string
		if i < 9 {
			key = "alt-" + string('1'+i)
		} else if i < 35 {
			key = "alt-" + string('a'+i-9)
		}
		paneRooms.SetCell(i, 0, tview.NewTableCell("[darkcyan]"+key+"[white]").SetExpansion(1))
		paneRooms.SetCell(i, 1, tview.NewTableCell(room.User.Name).SetExpansion(2))
		paneRooms.SetCell(i, 2, tview.NewTableCell(room.CurrentUsersCount).SetExpansion(2))
		paneRooms.SetCell(i, 3, tview.NewTableCell(room.Name))
	}
	if paneRoomsLoading != nil {
		grid.RemoveItem(paneRoomsLoading)
		paneRoomsLoading = nil
	}
	grid.AddItem(paneRooms,
		1, 1, // row, column
		1, 1, // rowSpan, colSpan
		0, 0, // minGridHeight, minGridWidth
		false) // focus
	app.Draw()
	app.SetFocus(paneRooms)
}

func selectRoom(index int) {
	if index < 0 || index >= len(rooms) {
		return
	}
	stream := rooms[index].StreamUrl
	web := rooms[index].WebUrl
	if _, err := exec.LookPath("open"); err == nil {
		if lastEnterWithAlt == false && videoPlayer != "" {
			exec.Command("open", "-na", videoPlayer, stream).Start()
			return
		}
		exec.Command("open", web).Start()
	}
}

func selectCategory(cat *dylive.Category) {
	var pane tview.Primitive

	if cat == nil {
		pane = paneSubCatsLoading
	} else {
		if paneSubCatsLoading != nil {
			grid.RemoveItem(paneSubCatsLoading)
			paneSubCatsLoading = nil
		}

		if paneSubCats != nil {
			grid.RemoveItem(paneSubCats)
		}

		paneSubCats = tview.NewList().
			SetHighlightFullLine(true).
			SetWrapAround(false).
			SetShortcutColor(tcell.ColorDarkCyan).
			ShowSecondaryText(false)

		pane = paneSubCats

		var firstHandler func()
		for i, subcat := range cat.Categories {
			var key rune
			if i < 26 {
				key = 'a' + rune(i)
			} else if i < 52 {
				key = 'A' + rune(i-26)
			} else if i < 52+len(extraKeys) {
				key = rune(extraKeys[i-52])
			}
			id := subcat.Id
			name := subcat.Name
			handler := func() {
				go getRooms(id, name)
			}
			if firstHandler == nil {
				firstHandler = handler
			}
			paneSubCats.AddItem(name, "", key, handler)
		}
		if firstHandler != nil {
			firstHandler()
		}
	}

	grid.AddItem(pane,
		1, 0, // row, column
		1, 1, // rowSpan, colSpan
		0, 0, // minGridHeight, minGridWidth
		false) // focus
}

func onKeyPressed(event *tcell.EventKey) *tcell.EventKey {
	r := event.Rune()
	if event.Modifiers() == tcell.ModAlt {
		if r >= '1' && r <= '9' {
			idx := int(r - '1')
			paneRooms.Select(idx, 0)
			selectRoom(idx)
		} else if r >= 'a' && r <= 'z' {
			idx := int(r-'a') + 9
			paneRooms.Select(idx, 0)
			selectRoom(idx)
		}
	} else {
		if r >= '1' && r <= '9' {
			n := int(r - '0')
			if n >= 1 && n <= len(categories) {
				paneCats.Highlight(strconv.Itoa(n))
			}
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || strings.ContainsRune(extraKeys, r) {
			app.SetFocus(paneSubCats)
		}
	}
	switch event.Key() {
	case tcell.KeyEnter:
		lastEnterWithAlt = event.Modifiers() == tcell.ModAlt
	case tcell.KeyLeft, tcell.KeyBacktab:
		if n := getCurrentCategoryNumber(); n > 1 {
			paneCats.Highlight(strconv.Itoa(n - 1))
		}
	case tcell.KeyRight, tcell.KeyTab:
		if n := getCurrentCategoryNumber(); n < len(categories) {
			paneCats.Highlight(strconv.Itoa(n + 1))
		}
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn:
		if event.Modifiers() == tcell.ModAlt {
			app.SetFocus(paneSubCats)
		} else {
			app.SetFocus(paneRooms)
		}
	}
	return event
}

func findVideoPlayer() string {
	if _, err := os.Stat("/Applications/IINA.app"); !os.IsNotExist(err) {
		return "IINA"
	} else if _, err := os.Stat("/Applications/VLC.app"); !os.IsNotExist(err) {
		return "VLC"
	}
	return ""
}

func getHelp() string {
	vp := videoPlayer
	if vp == "" {
		vp = "默认程序"
	}
	return fmt.Sprintf("[darkcyan]Alt+Up/Down[white] 切换分类  "+
		"[darkcyan]Enter[white] 在%s打开  "+
		"[darkcyan]Alt-Enter[white] 在浏览器打开", vp)
}

type verticalText struct {
	*tview.TextView
}

func (t *verticalText) Draw(s tcell.Screen) {
	_, _, _, height := t.TextView.GetRect()
	t.TextView.SetBorderPadding(height/2, 0, 0, 0)
	t.TextView.Draw(s)
}
