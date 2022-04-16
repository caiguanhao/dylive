# dylive

Utility to watch Douyin live streams. 观看抖音直播工具

- Use keyboard or mouse to navigate different categories.
- Select multiple live stream rooms and open them at once.

To install:

You must have installed [Go](https://go.dev/dl/) first.

NOTE: Douyin has removed many online game live broadcasting categories on April 16, 2022.

```
go install -v github.com/caiguanhao/dylive/dylive@latest
```

Also install [mpv](https://mpv.io/installation/) to play live stream.

Works on macOS, Linux or Windows.

Preview:

https://user-images.githubusercontent.com/1284703/147945918-a20c6c96-88d7-46b6-834e-1650b8033605.mp4

## Usage

### Video player

By default, dylive uses mpv. If mpv does not exist, IINA and then VLC will be
used. If you have installed video player in a different location, set the
`PLAYER` environment variable.

```
# use different video player command
PLAYER=ffplay dylive

# specify full path
PLAYER=/Applications/IINA.app/Contents/MacOS/iina-cli dylive

# also works for iPhone video player on Apple Silicon Macs
PLAYER=/Applications/OPlayer\ Lite.app dylive
```

### Video player arguments

You can add extra video player command arguments after dylive like this:

```
# mute first when starting mpv
dylive -- --mute=yes
```

See [mpv's options](https://mpv.io/manual/master/#options).

### Record while playing

Use mpv's `--stream-record` option to save live stream to file.

You can use template in player arguments.

List of variables that can be used in template:
- Room info (which can be obtained with `Ctrl-E`)
- Index number of room (`{{.Index}}` or `{{.Nth}}`)
- Number of rooms (`{{.Total}}`)
- Current date/time (`{{.Now}}`), its format can be changed with `TIME_FORMAT` environment variable.

```
# assume you have mpv command in your PATH
dylive -- --stream-record={{.User.Name}}.mp4

# for Windows, you may need to add quotes
dylive -- --no-border "--stream-record={{.User.Name}}-{{.Now}}.mp4"
```

Press `Ctrl-S` to view list of commands.
