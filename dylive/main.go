package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path"
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

	selectedRooms selections

	lastEnterWithAlt bool

	currentCat    *dylive.Category
	currentHelp   int = -1
	currentConfig config

	statusChan = make(chan status)

	helps = []string{
		"[darkcyan](Shift)+Tab[white] 切换主分类",
		"[darkcyan]Alt+Up/Down[white] 切换子分类",
		"[darkcyan]Space[white] 选择多个直播",
		"[darkcyan]Alt+Space[white] 反向选择",
		"[darkcyan]Enter[white] 播放器打开",
		"[darkcyan]Alt-Enter[white] 浏览器打开",
		"[darkcyan]Ctrl-(Alt)-E[white] 编辑器打开",
	}
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
				currentCat = &categories[idx]
				selectCategory()
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
			if len(selectedRooms) > 0 {
				selectRooms()
			} else {
				selectRoomByIndex(row)
			}
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

	currentCat = nil
	selectCategory()

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
	paneRooms.Select(0, 0)
	renderRooms()
	app.Draw()
	app.SetFocus(paneRooms)
}
func renderRooms() {
	paneRooms.Clear()
	for i, room := range rooms {
		var key string
		if selectedRooms.has(room) {
			key = "[cyan]" + tview.Escape("[X]") + "[white]"
		} else if i < 9 {
			key = "[darkcyan](" + string('1'+byte(i)) + ")[white]"
		} else {
			key = "   "
		}
		name := key + " " + room.User.Name
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

func selectRooms() {
	size := len(selectedRooms)
	for i, room := range selectedRooms {
		selectRoom(room, i, size)
	}
}

func selectRoomByIndex(index int) {
	if index < 0 || index >= len(rooms) {
		return
	}
	room := rooms[index]
	selectRoom(room, 0, 0)
}

func selectRoom(room dylive.Room, nth, total int) {
	if lastEnterWithAlt == true {
		exec.Command("open", room.WebUrl).Start()
		return
	}

	var cmdType, cmdName string
	var cmd *exec.Cmd

	player := os.Getenv("PLAYER")
	if player == "" {
		if commandExists("mpv") {
			cmdType = "mpv"
			cmdName = "mpv"
		} else if commandExists("iina-cli") {
			cmdType = "iina"
			cmdName = "iina-cli"
		} else if exists("/Applications/IINA.app") {
			cmdType = "open"
			cmdName = "IINA"
		} else if exists("/Applications/VLC.app") {
			cmdType = "open"
			cmdName = "VLC"
		}
	} else {
		cmdName = player
		base := path.Base(player)
		if base == "mpv" {
			cmdType = "mpv"
		} else if base == "iina-cli" {
			cmdType = "iina"
		} else if strings.HasSuffix(base, ".app") && isDir(player) {
			cmdType = "open"
			cmdName = player
		}
	}

	switch cmdType {
	case "mpv":
		args := []string{"--title=" + room.User.Name, "--force-window=immediate", room.StreamUrl}
		if geometry := mpvGeometry(nth, total); geometry != "" {
			args = append(args, "--geometry="+geometry)
		}
		cmd = exec.Command(cmdName, args...)
	case "iina":
		args := []string{room.StreamUrl, "--", "--force-media-title=" + room.User.Name}
		if geometry := mpvGeometry(nth, total); geometry != "" {
			args = append(args, "--geometry="+geometry)
			time.Sleep(500 * time.Millisecond) // iina bug? open too fast will break
		}
		cmd = exec.Command(cmdName, args...)
		cmd.Stdin = os.Stdin
	case "open":
		cmd = exec.Command("open", "-na", cmdName, room.StreamUrl)
	default:
		if cmdName != "" {
			cmd = exec.Command(cmdName, room.StreamUrl)
		}
	}

	if cmd != nil {
		cmd.Start()
	}
}

func selectCategory() {
	var pane tview.Primitive

	if currentCat == nil {
		pane = paneSubCatsLoading
	} else {
		currentConfig.DefaultCategory = currentCat.Name

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

		firstHandler := renderSubcats(false)
		if firstHandler != nil {
			firstHandler()
		}

		pane = paneSubCats
	}

	grid.AddItem(pane,
		1, 0, // row, column
		1, 1, // rowSpan, colSpan
		0, 0, // minGridHeight, minGridWidth
		false) // focus
}

func renderSubcats(keepCurrentSelection bool) (firstHandler func()) {
	idx := paneSubCats.GetCurrentItem()
	paneSubCats.Clear()
	for i, subcat := range currentCat.Categories {
		var key rune
		if i < 26 {
			key = 'a' + rune(i)
		} else if i < 52 {
			key = 'A' + rune(i-26)
		} else if i < 52+len(extraKeys) {
			key = rune(extraKeys[i-52])
		}
		id := subcat.Id
		count := selectedRooms.count(subcat.Name)
		var name string
		if count > 0 {
			name = fmt.Sprintf("%s (%d)", subcat.Name, count)
		} else {
			name = subcat.Name
		}
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
	if keepCurrentSelection {
		paneSubCats.SetCurrentItem(idx)
	}
	return
}

func onKeyPressed(event *tcell.EventKey) *tcell.EventKey {
	r := event.Rune()
	key := event.Key()
	if r == '?' {
		nextHelpMessage()
		return nil
	}
	if r == ' ' {
		if event.Modifiers()&tcell.ModAlt != 0 {
			for _, room := range rooms {
				selectedRooms.toggle(room)
			}
			renderRooms()
			renderSubcats(true)
			return nil
		}
		row, _ := paneRooms.GetSelection()
		if row < 0 || row >= len(rooms) {
			return nil
		}
		selectedRooms.toggle(rooms[row])
		renderRooms()
		renderSubcats(true)
		return nil
	}
	if key >= tcell.KeyF1 && key <= tcell.KeyF12 {
		n := int(key-tcell.KeyF1) + 1
		if n >= 1 && n <= len(categories) {
			paneCats.Highlight(strconv.Itoa(n))
		}
		return nil
	}
	if r >= '1' && r <= '9' {
		idx := int(r - '1')
		paneRooms.Select(idx, 0)
		selectRoomByIndex(idx)
		return nil
	}
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || strings.ContainsRune(extraKeys, r) {
		app.SetFocus(paneSubCats)
		return event
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
				if commandExists("vim") {
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

func exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func isDir(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

type selections []dylive.Room

func (s *selections) toggle(i dylive.Room) {
	if s.has(i) {
		n := selections{}
		for _, a := range *s {
			if a.User.Name != i.User.Name {
				n = append(n, a)
			}
		}
		*s = n
	} else {
		*s = append(*s, i)
	}
}

func (s selections) has(i dylive.Room) bool {
	for _, a := range s {
		if a.User.Name == i.User.Name {
			return true
		}
	}
	return false
}

func (s selections) count(subCatName string) int {
	total := 0
outer:
	for _, a := range s {
		for _, c := range a.Category.Categories {
			if c.Name == subCatName {
				total += 1
				continue outer
			}
		}
	}
	return total
}

func arrange(size int) (rows, cols int) {
	x := math.Sqrt(float64(size))
	rows = int(math.Round(x))
	cols = int(math.Ceil(float64(size) / math.Round(x)))
	return
}

func mpvGeometry(nth, size int) string {
	if size == 0 {
		return ""
	}
	rows, cols := arrange(size)
	cp := int(math.Floor(100 / float64(cols-1)))
	rp := int(math.Floor(100 / float64(rows-1)))
	w := int(math.Floor(100 / float64(cols)))
	x := nth % cols * cp
	y := nth / cols * rp
	return fmt.Sprintf("%d%%+%d%%+%d%%", w, x, y)
}
