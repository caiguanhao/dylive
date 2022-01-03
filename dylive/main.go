package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
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
	paneHelp    *tview.TextView

	paneSubCatsLoading *tview.TextView
	paneRoomsLoading   *tview.TextView

	paneCatsShowKeys      bool
	paneRoomsShowRoomName bool

	categories []dylive.Category
	rooms      []dylive.Room

	lastEnterWithAlt bool

	videoPlayer   string
	helps         []string
	currentHelp   int = -1
	currentConfig config

	statusChan = make(chan status)
)

const (
	title     = "dylive"
	extraKeys = `!@#$%^&*()-=[]\;',./_+{}|:"<>`
)

type config struct {
	DefaultCategory    string
	DefaultSubCategory string
}

func main() {
	defaultConfigFile := ".dylive.json"
	if home, _ := os.UserHomeDir(); home != "" {
		defaultConfigFile = filepath.Join(home, defaultConfigFile)
	}
	configFile := flag.String("c", defaultConfigFile, "config file location")
	flag.Parse()

	cfdata, _ := ioutil.ReadFile(*configFile)
	json.Unmarshal(cfdata, &currentConfig)

	videoPlayer = findVideoPlayer()
	helps = getHelpMessages()

	app = tview.NewApplication()

	app.SetInputCapture(onKeyPressed)

	paneCats = tview.NewTextView().
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
	paneCats.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
		if showKeys := w > 75; paneCatsShowKeys != showKeys {
			paneCatsShowKeys = showKeys
			renderCategories()
		}
		return x, y, w, h
	})

	paneStatus = tview.NewTextView()
	paneStatus.SetBorderPadding(0, 0, 1, 1)
	go monitorStatus()

	paneHelp = tview.NewTextView().
		SetTextAlign(tview.AlignRight).
		SetDynamicColors(true)
	paneHelp.SetBorderPadding(0, 0, 1, 1)
	nextHelpMessage()

	paneFooter := tview.NewFlex().
		AddItem(paneStatus, 0, 1, false).
		AddItem(paneHelp, 0, 1, false)

	paneRooms = tview.NewTable().
		SetSelectable(true, false).
		SetBorders(false).
		SetSelectionChangedFunc(func(row, column int) {
			if row < 0 || row >= len(rooms) {
				return
			}
			go updateStatus(rooms[row].WebUrl, 0)
		}).
		SetSelectedFunc(func(row, column int) {
			selectRoom(row)
		})
	paneRooms.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
		if showRoomName := w > 60; paneRoomsShowRoomName != showRoomName {
			paneRoomsShowRoomName = showRoomName
			renderRooms()
		}
		return x, y, w, h
	})

	paneSubCatsLoading = tview.NewTextView().SetText("正在载入…").SetTextAlign(tview.AlignCenter)
	paneSubCatsLoading.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
		y += h / 2
		return x, y, w, h
	})
	paneRoomsLoading = tview.NewTextView().SetText("正在载入…").SetTextAlign(tview.AlignCenter)
	paneRoomsLoading.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
		y += h / 2
		return x, y, w, h
	})

	grid = tview.NewGrid().
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

	grid.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
		grid.SetRows(1, 0, 1)
		w1 := w / 3
		if w1 > 30 {
			w1 = 30
		}
		grid.SetColumns(w1, 0)
		return x, y, w, h
	})

	selectCategory(nil)

	go getCategories()

	if err := app.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}

	cfdata, _ = json.MarshalIndent(currentConfig, "", "  ")
	ioutil.WriteFile(*configFile, cfdata, 0644)
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
	go updateStatus("正在获取分类…", 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	categories, err = dylive.GetCategories(ctx)
	if err != nil {
		go updateStatus(err.Error(), 0)
		return
	}
	go updateStatus("成功获取分类", 0)
	renderCategories()
	n := "1"
	for i, cat := range categories {
		if cat.Name == currentConfig.DefaultCategory {
			n = strconv.Itoa(i + 1)
		}
	}
	paneCats.Highlight(n)
	app.Draw()
}

func renderCategories() {
	if len(categories) == 0 {
		paneCats.SetText(title)
		return
	}
	paneCats.Clear()
	for i, cat := range categories {
		if i > 0 {
			fmt.Fprintf(paneCats, "  ")
		}
		if paneCatsShowKeys {
			fmt.Fprintf(paneCats, `F%d `, i+1)
		}
		fmt.Fprintf(paneCats, `["%d"][darkcyan]%s[white][""]`, i+1, cat.Name)
	}
}

func getRooms(id, name string) {
	go updateStatus(fmt.Sprintf("正在获取「%s」的直播列表…", name), 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	rooms, err = dylive.GetRoomsByCategory(ctx, id)
	if err != nil {
		go updateStatus(err.Error(), 0)
		return
	}
	currentConfig.DefaultSubCategory = name
	go updateStatus(fmt.Sprintf("成功获取「%s」的直播列表", name), 1*time.Second)
	renderRooms()
	app.Draw()
	app.SetFocus(paneRooms)
}
func renderRooms() {
	paneRooms.Clear()
	paneRooms.Select(0, 0)
	for i, room := range rooms {
		var key string
		if i < 9 {
			key = "[darkcyan](" + string('1'+i) + ")[white] "
		} else {
			key = "    "
		}
		name := key + room.User.Name
		paneRooms.SetCell(i, 0, tview.NewTableCell(name).SetExpansion(2))
		paneRooms.SetCell(i, 1, tview.NewTableCell(room.CurrentUsersCount).SetExpansion(2))
		if paneRoomsShowRoomName {
			paneRooms.SetCell(i, 2, tview.NewTableCell(room.Name))
		}
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
		currentConfig.DefaultCategory = cat.Name

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
			if name == currentConfig.DefaultSubCategory {
				paneSubCats.SetCurrentItem(i)
				firstHandler = handler
			}
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
	key := event.Key()
	if r == '?' {
		nextHelpMessage()
		return event
	}
	if key >= tcell.KeyF1 && key <= tcell.KeyF12 {
		n := int(key-tcell.KeyF1) + 1
		if n >= 1 && n <= len(categories) {
			paneCats.Highlight(strconv.Itoa(n))
		}
	}
	if r >= '1' && r <= '9' {
		idx := int(r - '1')
		paneRooms.Select(idx, 0)
		selectRoom(idx)
	}
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || strings.ContainsRune(extraKeys, r) {
		app.SetFocus(paneSubCats)
	}
	switch key {
	case tcell.KeyCtrlE:
		app.Suspend(func() {
			var data interface{}
			if event.Modifiers()&tcell.ModAlt != 0 {
				data = rooms
			} else {
				row, _ := paneRooms.GetSelection()
				if row < 0 || row >= len(rooms) {
					return
				}
				data = rooms[row]
			}
			file, err := ioutil.TempFile("", "dylive")
			if err != nil {
				return
			}
			defer os.Remove(file.Name())
			defer file.Close()
			e := json.NewEncoder(file)
			e.SetIndent("", "  ")
			e.SetEscapeHTML(false)
			e.Encode(data)
			editor := os.Getenv("EDITOR")
			if editor == "" {
				if _, err := exec.LookPath("vim"); err == nil {
					editor = "vim"
				} else {
					editor = "vi"
				}
			}
			cmd := exec.Command(editor, file.Name())
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Run()
		})
		return nil
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

func getHelpMessages() []string {
	vp := videoPlayer
	if vp == "" {
		vp = "默认程序"
	}
	return []string{
		"[darkcyan](Shift)+Tab[white] 切换主分类",
		"[darkcyan]Alt+Up/Down[white] 切换子分类",
		fmt.Sprintf("[darkcyan]Enter[white] 在%s打开", vp),
		"[darkcyan]Alt-Enter[white] 在浏览器打开",
		"[darkcyan]Ctrl-(Alt)-E[white] 在编辑器打开",
	}
}

func nextHelpMessage() {
	paneHelp.Clear()
	if currentHelp > -1 {
		fmt.Fprint(paneHelp, helps[currentHelp], "  ")
		fmt.Fprint(paneHelp, `[darkcyan]?[white] 下个帮助`)
	} else {
		fmt.Fprint(paneHelp, `[darkcyan]?[white] 显示帮助`)
	}
	currentHelp += 1
	if currentHelp >= len(helps) {
		currentHelp = -1
	}
}

type status struct {
	text string
	wait time.Duration
}

func monitorStatus() {
	for {
		select {
		case status := <-statusChan:
			paneStatus.SetText(status.text)
			app.Draw()
			time.Sleep(status.wait)
		}
	}
}

func updateStatus(text string, wait time.Duration) {
	statusChan <- status{text: text, wait: wait}
}
