package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unsafe"

	"github.com/caiguanhao/dylive"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	app   *tview.Application
	grid  *tview.Grid
	pages *tview.Pages

	paneCats    *tview.TextView
	paneSubCats *tview.List
	paneRooms   *tview.Table
	paneStatus  *tview.TextView
	paneHelp    *tview.TextView

	paneSubCatsLoading *tview.TextView
	paneRoomsLoading   *tview.TextView

	paneCatsShowKeys      bool
	paneRoomsShowRoomName bool
	paneRoomsX            int

	categories []dylive.Category
	rooms      []dylive.Room

	selectedRooms selections

	lastEnterWithAlt bool
	lastMouseClick   time.Time

	currentCat    *dylive.Category
	currentSubCat *dylive.Category
	currentHelp   int = -1
	currentConfig config
	preferQuality string

	color      = "lightgreen"
	isWindows  = runtime.GOOS == "windows"
	borderless = isWindows
	timeFormat = "2006-01-02-15-04-05"

	statusChan = make(chan status)

	helps = [][]string{
		{"(Shift)+Tab", "切换主分类"},
		{"Alt+Up/Down/PgUp/PgDn", "切换子分类"},
		{"Space", "选择多个直播"},
		{"Ctrl+A", "当前页反向选择"},
		{"Backspace", "取消所有选择"},
		{"Enter", "播放器中打开"},
		{"Alt-Enter", "浏览器中打开"},
		{"Ctrl+(Alt)+E", "编辑器中查看信息"},
		{"Ctrl-S", "编辑器中查看命令"},
		{"Ctrl+(Alt)+R", "重新加载"},
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
	noMouse := flag.Bool("no-mouse", false, "disable mouse")
	flag.StringVar(&preferQuality, "q", "hd", "video quality (uhd, hd, ld, sd)")
	flag.Usage = func() {
		o := flag.CommandLine.Output()
		fmt.Fprintln(o, "Usage:", filepath.Base(os.Args[0]), "[options] -- [player arguments]")
		flag.PrintDefaults()
		fmt.Fprintln(o)
		fmt.Fprintln(o, "EnvVars:")
		fmt.Fprintln(o, "  PLAYER - video player, defaults to mpv and iina-cli")
		fmt.Fprintln(o, "  EDITOR - text editor, defaults to vim or vi")
		fmt.Fprintln(o, "  COLOR  - color for keys, defaults to", color)
		fmt.Fprintln(o, "           https://github.com/gdamore/tcell/blob/v2.4.0/color.go#L845")
		fmt.Fprintln(o, "  TIME_FORMAT - time format, defaults to", timeFormat)
		fmt.Fprintln(o, "                https://pkg.go.dev/time#pkg-constants")
		fmt.Fprintln(o)
		fmt.Fprintln(o, "Keys:")
		for _, h := range helps {
			fmt.Fprintf(o, "  %-12s - %s\n", h[0], h[1])
		}
	}
	flag.Parse()

	if c := os.Getenv("COLOR"); c != "" {
		color = c
	}
	if tf := os.Getenv("TIME_FORMAT"); tf != "" {
		timeFormat = tf
	}

	cfdata, _ := ioutil.ReadFile(*configFile)
	json.Unmarshal(cfdata, &currentConfig)

	tview.DoubleClickInterval = 300 * time.Millisecond

	app = tview.NewApplication()
	app.EnableMouse(!*noMouse)
	app.SetInputCapture(onKeyPressed)

	createCategories()
	createStatus()
	createHelp()
	createRooms()

	paneFooter := tview.NewFlex().
		AddItem(paneStatus, 0, 1, false).
		AddItem(paneHelp, 0, 1, false)

	grid = tview.NewGrid().
		SetBorders(!borderless).
		AddItem(paneCats,
			0, 0, // row, column
			1, 2, // rowSpan, colSpan
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

	reset()

	go getCategories()

	pages = tview.NewPages()
	pages.AddPage("grid", grid, true, true)

	if err := app.SetRoot(pages, true).Run(); err != nil {
		panic(err)
	}

	cfdata, _ = json.MarshalIndent(currentConfig, "", "  ")
	ioutil.WriteFile(*configFile, cfdata, 0644)
}

func reset() {
	if paneSubCats != nil {
		grid.RemoveItem(paneSubCats)
		paneSubCats = nil
	}

	if paneRooms != nil {
		grid.RemoveItem(paneRooms)
	}

	if paneSubCatsLoading != nil {
		grid.RemoveItem(paneSubCatsLoading)
	}
	paneSubCatsLoading = tview.NewTextView().SetText("正在载入…").SetTextAlign(tview.AlignCenter)
	paneSubCatsLoading.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
		y += h / 2
		return x, y, w, h
	})

	if paneRoomsLoading != nil {
		grid.RemoveItem(paneRoomsLoading)
	}
	paneRoomsLoading = tview.NewTextView().SetText("正在载入…").SetTextAlign(tview.AlignCenter)
	paneRoomsLoading.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
		y += h / 2
		return x, y, w, h
	})

	categories = nil
	renderCategories()
	paneCats.Highlight("0")

	currentCat = nil
	currentSubCat = nil
	selectCategory()

	grid.AddItem(paneRoomsLoading,
		1, 1, // row, column
		1, 1, // rowSpan, colSpan
		0, 0, // minGridHeight, minGridWidth
		false) // focus
}

func createCategories() {
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
}

func getCategories() {
	go updateStatus("正在获取分类…", 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	categories, err = dylive.GetCategories(ctx)
	if err != nil {
		go showError(err)
		return
	}

	gameIdx := -1
	for i, c := range categories {
		if c.Name == "游戏" {
			gameIdx = i
			break
		}
	}
	if gameIdx != -1 {
		game := categories[gameIdx]
		others := make([]dylive.Category, 0, len(categories)-1)
		for i, c := range categories {
			if i == gameIdx {
				continue
			}
			others = append(others, c)
		}
		newCategories := make([]dylive.Category, 0, len(game.Categories)+1)
		newCategories = append(newCategories, game.Categories...)
		newCategories = append(newCategories, dylive.Category{
			Name:       "其他",
			Categories: others,
		})
		categories = newCategories
	}

	go updateStatus("成功获取分类", 0)
	app.QueueUpdateDraw(func() {
		renderCategories()
		n := "1"
		for i, cat := range categories {
			if cat.Name == currentConfig.DefaultCategory {
				n = strconv.Itoa(i + 1)
			}
		}
		paneCats.Highlight(n)
	})
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
		fmt.Fprintf(paneCats, `["%d"][%s]%s[white][""]`, i+1, color, cat.Name)
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
			SetShortcutColor(tcell.GetColor(color)).
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

func renderSubcats(keepCurrentSelection bool) (firstHandler func()) {
	idx := paneSubCats.GetCurrentItem()
	paneSubCats.Clear()
	for i := range currentCat.Categories {
		var key rune
		if i < 26 {
			key = 'a' + rune(i)
		} else if i < 52 {
			key = 'A' + rune(i-26)
		} else if i < 52+len(extraKeys) {
			key = rune(extraKeys[i-52])
		}
		subcat := currentCat.Categories[i]
		count := selectedRooms.count(subcat.Name)
		var name string
		if count > 0 {
			name = fmt.Sprintf("%s (%d)", subcat.Name, count)
		} else {
			name = subcat.Name
		}
		handler := func() {
			currentSubCat = &subcat
			go getRooms()
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

func createRooms() {
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
		if borderless {
			x += 1
			w -= 1
		}
		paneRoomsX = x
		return x, y, w, h
	})
	paneRooms.SetMouseCapture(onRoomsClicked)
}

func getRooms() {
	if currentSubCat == nil {
		return
	}
	if paneRooms != nil {
		paneRooms.Clear()
	}
	go updateStatus(fmt.Sprintf("正在获取「%s」的直播列表…", currentSubCat.Name), 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	rooms, err = dylive.GetRoomsByCategory(ctx, currentSubCat.Id)
	if err != nil {
		go showError(err)
		return
	}
	currentConfig.DefaultSubCategory = currentSubCat.Name
	go updateStatus("成功获取", 1*time.Second)
	app.QueueUpdateDraw(func() {
		paneRooms.Select(0, 0)
		renderRooms()
		app.SetFocus(paneRooms)
	})
}

func renderRooms() {
	paneRooms.Clear()
	for i, room := range rooms {
		var key string
		if selectedRooms.has(room) {
			key = "[" + color + "]" + tview.Escape("[X]") + "[white]"
		} else if i < 9 {
			key = "[" + color + "](" + string('1'+byte(i)) + ")[white]"
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
	if count := len(selectedRooms); count > 9 {
		modal := newModal()
		modal.SetText(fmt.Sprintf("确定要打开 %d 个直播吗？", count)).
			AddButtons([]string{
				"确定",
				"取消",
			}).SetFocus(1).SetDoneFunc(func(index int, label string) {
			if index == 0 {
				selectRoomsForce()
			}
			pages.RemovePage("modal")
			app.SetFocus(paneRooms)
		})
		pages.AddPage("modal", modal, false, false)
		pages.ShowPage("modal")
		return
	}
	selectRoomsForce()
}

func selectRoomsForce() {
	size := len(selectedRooms)
	for i, room := range selectedRooms {
		selectRoom(room, i, size, false)
	}
}

func selectRoomByIndex(index int) {
	if index < 0 || index >= len(rooms) {
		return
	}
	room := rooms[index]
	selectRoom(room, 0, 0, false)
}

func roomsCommands() (cmds []string) {
	size := len(selectedRooms)
	if size == 0 {
		row, _ := paneRooms.GetSelection()
		if row < 0 || row >= len(rooms) {
			return
		}
		cmds = append(cmds, selectRoom(rooms[row], 0, 1, true).String())
		return
	}
	for i, room := range selectedRooms {
		cmds = append(cmds, selectRoom(room, i, size, true).String())
	}
	return
}

func selectRoom(room dylive.Room, nth, total int, noRun bool) *exec.Cmd {
	if lastEnterWithAlt == true {
		var cmd *exec.Cmd
		if isWindows {
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", room.WebUrl)
		} else if commandExists("xdg-open") {
			cmd = exec.Command("xdg-open", room.WebUrl)
		} else {
			cmd = exec.Command("open", room.WebUrl)
		}
		if noRun == false {
			cmd.Start()
		}
		return cmd
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
		base := filepath.Base(player)
		if base == "mpv" || base == "mpv.exe" {
			cmdType = "mpv"
		} else if base == "iina-cli" {
			cmdType = "iina"
		} else if strings.HasSuffix(base, ".app") && isDir(player) {
			cmdType = "open"
			cmdName = player
		}
	}

	url := room.FlvUrlForQuality(preferQuality)

	switch cmdType {
	case "mpv":
		args := []string{"--title=" + room.User.Name, "--force-window=immediate", url}
		if geometry := mpvGeometry(nth, total); geometry != "" {
			args = append(args, "--geometry="+geometry)
		}
		args = append(args, playerArgs(room, nth, total)...)
		cmd = exec.Command(cmdName, args...)
	case "iina":
		args := []string{url, "--", "--force-media-title=" + room.User.Name}
		if geometry := mpvGeometry(nth, total); geometry != "" {
			args = append(args, "--geometry="+geometry)
			time.Sleep(500 * time.Millisecond) // iina bug? open too fast will break
		}
		args = append(args, playerArgs(room, nth, total)...)
		cmd = exec.Command(cmdName, args...)
		cmd.Stdin = os.Stdin
	case "open":
		cmd = exec.Command("open", "-na", cmdName, url)
	default:
		if cmdName != "" {
			args := []string{url}
			args = append(args, playerArgs(room, nth, total)...)
			cmd = exec.Command(cmdName, args...)
		}
	}

	if noRun == false && cmd != nil {
		cmd.Start()
	}

	return cmd
}

func playerArgs(room dylive.Room, nth, total int) (out []string) {
	obj := struct {
		dylive.Room
		Index int
		Nth   int
		Total int
		Now   string
	}{room, nth, nth + 1, total, time.Now().Format(timeFormat)}
	for _, arg := range flag.Args() {
		tpl, err := template.New("").Parse(arg)
		if err != nil {
			out = append(out, arg)
			continue
		}
		var buf bytes.Buffer
		if tpl.Execute(&buf, obj) != nil {
			out = append(out, arg)
			continue
		}
		out = append(out, buf.String())
	}
	return
}

func onRoomsClicked(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
	when := event.When()
	if when.Sub(lastMouseClick) < 100*time.Millisecond {
		return action, event
	}
	switch action {
	case tview.MouseMiddleClick:
		go app.QueueUpdateDraw(func() {
			lastEnterWithAlt = true
			row, _ := paneRooms.GetSelection()
			selectRoomByIndex(row)
			lastMouseClick = when
			lastEnterWithAlt = false
		})
		return tview.MouseLeftClick, event
	case tview.MouseLeftClick:
		x, _ := event.Position()
		if x-paneRoomsX < 4 { // click on number
			go app.QueueUpdateDraw(func() {
				row, _ := paneRooms.GetSelection()
				if row < 0 || row >= len(rooms) {
					return
				}
				selectedRooms.toggle(rooms[row])
				renderRooms()
				renderSubcats(true)
				lastMouseClick = when
			})
		}
	case tview.MouseLeftDoubleClick:
		if len(selectedRooms) > 0 {
			selectRooms()
		} else {
			row, _ := paneRooms.GetSelection()
			selectRoomByIndex(row)
		}
		lastMouseClick = when
		return action, nil
	}
	return action, event
}

func onKeyPressed(event *tcell.EventKey) *tcell.EventKey {
	if pages.HasPage("modal") {
		return event
	}
	r := event.Rune()
	key := event.Key()
	if r == '?' {
		nextHelpMessage()
		return nil
	}
	if r == ' ' {
		if event.Modifiers()&tcell.ModAlt != 0 {
			invertSelection()
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
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		selectedRooms = nil
		renderRooms()
		renderSubcats(true)
		return nil
	case tcell.KeyCtrlA:
		invertSelection()
		return nil
	case tcell.KeyCtrlR:
		if event.Modifiers()&tcell.ModAlt != 0 || currentSubCat == nil {
			forceReload()
			return nil
		}
		go getRooms()
		return nil
	case tcell.KeyCtrlS:
		suspend(func() {
			editInEditor(func(file *os.File) {
				fmt.Fprintln(file, "#!/bin/bash")
				fmt.Fprintln(file)
				if len(selectedRooms) > 0 {
					fmt.Fprintln(file, "#", len(selectedRooms), "个直播")
				}
				fmt.Fprintln(file, "# 部分参数可能需要手动加双引号")
				fmt.Fprintln(file)
				fmt.Fprintln(file, strings.Join(roomsCommands(), "\n\n"))
			})
		})
		return nil
	case tcell.KeyCtrlE:
		suspend(func() {
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
			editInEditor(func(file *os.File) {
				e := json.NewEncoder(file)
				e.SetIndent("", "  ")
				e.SetEscapeHTML(false)
				e.Encode(data)
			})
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

func forceReload() {
	reset()
	go getCategories()
}

func showError(err error) {
	go updateStatus("发生错误", 0)
	app.QueueUpdateDraw(func() {
		modal := newModal()
		modal.SetText("发生错误：" + err.Error()).
			AddButtons([]string{
				"重新加载",
			}).SetFocus(0).SetDoneFunc(func(index int, label string) {
			pages.RemovePage("modal")
			go app.QueueUpdateDraw(func() {
				time.Sleep(500 * time.Millisecond)
				forceReload()
			})
		})
		pages.AddPage("modal", modal, false, false)
		pages.ShowPage("modal")
	})
}

func newModal() *tview.Modal {
	modal := tview.NewModal()
	modal.SetBackgroundColor(tcell.ColorBlue)
	modal.SetButtonBackgroundColor(tcell.ColorBlue)
	if borderless {
		// hack
		field := reflect.ValueOf(modal).Elem().FieldByName("frame")
		iFrame := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
		if frame, ok := iFrame.(*tview.Frame); ok {
			frame.SetBorder(false)
			frame.SetBorderPadding(2, 0, 0, 0)
		}
	}
	return modal
}

func editInEditor(f func(*os.File)) {
	file, err := ioutil.TempFile("", "dylive")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	defer file.Close()
	f(file)
	editor := os.Getenv("EDITOR")
	if editor == "" {
		if isWindows {
			editor = "notepad"
		} else {
			if commandExists("vim") {
				editor = "vim"
			} else {
				editor = "vi"
			}
		}
	}
	cmd := exec.Command(editor, file.Name())
	if !isWindows {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
	}
	cmd.Run()
}

func invertSelection() {
	for _, room := range rooms {
		selectedRooms.toggle(room)
	}
	renderRooms()
	renderSubcats(true)
}

func createHelp() {
	paneHelp = tview.NewTextView().
		SetTextAlign(tview.AlignRight).
		SetDynamicColors(true)
	if !borderless {
		paneHelp.SetBorderPadding(0, 0, 1, 1)
	}
	nextHelpMessage()
}

func nextHelpMessage() {
	paneHelp.Clear()
	if currentHelp > -1 {
		fmt.Fprint(paneHelp, "[", color, "]", helps[currentHelp][0], "[white] ", helps[currentHelp][1], "  ")
		fmt.Fprint(paneHelp, "[", color, "]?[white] 下个帮助")
	} else {
		fmt.Fprint(paneHelp, "[", color, "]?[white] 显示帮助")
	}
	currentHelp += 1
	if currentHelp >= len(helps) {
		currentHelp = -1
	}
}

func createStatus() {
	paneStatus = tview.NewTextView()
	if !borderless {
		paneStatus.SetBorderPadding(0, 0, 1, 1)
	}
	paneStatus.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick && strings.HasPrefix(paneStatus.GetText(true), "http") {
			lastEnterWithAlt = true
			row, _ := paneRooms.GetSelection()
			selectRoomByIndex(row)
			lastEnterWithAlt = false
		}
		return action, event
	})
	go monitorStatus()
}

type status struct {
	text string
	wait time.Duration
}

func monitorStatus() {
	for {
		select {
		case status := <-statusChan:
			app.QueueUpdateDraw(func() {
				paneStatus.SetText(status.text)
			})
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

// app.Suspend could cause freeze problem on Windows
func suspend(f func()) {
	if isWindows {
		f()
	} else {
		app.Suspend(f)
	}
}
