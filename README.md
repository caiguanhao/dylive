# dylive

Get live stream m3u8 URL from Douyin share messages and monitor users for new
live broadcasts.

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
