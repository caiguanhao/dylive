# dylive

Get live stream m3u8 URL from Douyin share messages and monitor users for new
live broadcasts.

You can:
- Watch (multiple) Douyin HD live streams on computer (without phone)
- Write your own script and get notified (or start recording) once new live
  stream is started

## Install

```
go get -v -u github.com/caiguanhao/dylive
```

## Usage

### Watch Live Stream

In any live stream room, click Share (分享) and select Copy Link (复制链接).

<img src="https://user-images.githubusercontent.com/1284703/121233565-554aa580-c8c5-11eb-97bf-28f25d96057c.jpg" width="300" />

For example, on macOS, paste the link to dylive and launch mpv:

```
pbpaste | dylive | xargs open -na mpv
```

You can use [streamlink](https://streamlink.github.io/) to download the live stream while watching it.

```
dylive https://v.douyin.com/exdfyjt/ | xargs -I X streamlink --player /Applications/mpv.app/Contents/MacOS/mpv -r video.ts X best
```

### Wait User's Live Stream

In any user profile page, click Share (分享主页) and select Copy Link (复制链接).

Monitor list of users. Once one of them starts new live stream, opens new mpv window:

```
# it's OK to just use ID in the URLs
dylive exJ1CqY exJk92q | xargs -n 1 -I X open -na mpv X --args --autofit="50%" 
```

## Execute Command

You can use the `-exec` option to run a command, especially useful for Windows.

```
# play and record the live stream
dylive -exec "streamlink --player mpv -r video.ts {{.LiveStreamUrl}} best" exdgtjK

# ... with a custom file name
dylive -exec "streamlink -r {{printf \"%s - %s.ts\" .User.Name \
  (.CreatedAt.Format \"2006-01-02\") | printf \"%q\"}} {{.LiveStreamUrl}} best" exdgtjK
```

The command can read the live stream info in JSON from standard input.
For example, open multiple live streams and tile the windows:

```
dylive -exec "bash cmd.sh" list-of-ids...
```

```
# cmd.sh
info=($(jq -r ".Index, .LiveStreamUrl, .User.Name"))
index=${info[0]}
url=${info[1]}
name=${info[2]}
mpv --really-quiet --title="$name" \
  --geometry=50%+$((index % 4 % 2 * 100))%+$((index / 2 % 2 * 100))% \
  $url &
```

## Device ID

Device ID is a number required by the Douyin's user-related APIs and will
become unusable after a while. By default, the `dylive` command will
automatically get one if it is not working. You can get your own device ID if
you know how to [intercept](https://mitmproxy.org/) your Douyin requests.

You can use `dylive -n 1` to directly get one.

## API

```go
import "github.com/caiguanhao/dylive/douyinapi"
```

Docs: <https://pkg.go.dev/github.com/caiguanhao/dylive/douyinapi>
