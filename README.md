# dylive

Get live stream m3u8 URL and monitor users for new live broadcasts.

You can:
- Watch (multiple) Douyin HD live streams in your favorite video player on
  your desktop (without phone)
- Write your own script and get notified (or start recording, etc.) once new
  live stream is started

## Install

```
go get -v -u github.com/caiguanhao/dylive

# if you want to search for Douyin users, also install dysearch
go get -v -u github.com/caiguanhao/dylive/dysearch
```

## Usage

### Search And Watch

Search top 2 Douyin live streamers whose name contains 红警 (Red Alert) and
open mpv to watch their live streams.

```
dysearch -L -F -n 2 红警 | xargs dylive | xargs -n1 open -na mpv
```

### Watch Live Stream

In any user profile page, copy user ID (user name) listed below user's nick name.

<img src="https://user-images.githubusercontent.com/1284703/124866056-59660200-dfee-11eb-8f98-05419cbe115f.jpg" width="400" />

Use the user ID as the argument of the command. For example:

```
dylive 1011694538 | xargs open -na mpv
```

You can use [streamlink](https://streamlink.github.io/) to download the live stream while watching it.

```
dylive 1011694538 | xargs -I X streamlink --player /Applications/mpv.app/Contents/MacOS/mpv -r video.ts X best
```

## Execute Command

You can use the `-exec` option to run a command, especially useful for Windows.

```
# play and record the live stream
dylive -exec "streamlink --player mpv -r video.ts {{.LiveStreamUrl}} best" ...

# ... with a custom file name
dylive -exec "streamlink -r {{printf \"%s - %s.ts\" .User.NickName \
  (.User.Room.CreatedAt.Format \"2006-01-02\") | printf \"%q\"}} {{.LiveStreamUrl}} best" ...
```

The command can read the live stream info in JSON from standard input.
For example, open multiple live streams and tile the windows:

```
dylive -exec "bash cmd.sh" list-of-ids...
```

```
# cmd.sh
info=($(jq -r ".Index%4%2*100, .Index/2%2*100, .LiveStreamUrl, .User.NickName"))
x=${info[0]}
y=${info[1]}
url=${info[2]}
name=${info[3]}
mpv --really-quiet --title="$name" --no-border --geometry="50%+$x%+$y%" $url &
```

https://user-images.githubusercontent.com/1284703/122740688-cc653e00-d2b6-11eb-86a8-0bffb9e33a7a.mp4

## API

```go
import "github.com/caiguanhao/dylive/douyinapi"
```

Docs: <https://pkg.go.dev/github.com/caiguanhao/dylive/douyinapi>
