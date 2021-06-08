# dylive

Get Douyin live stream URL from the live stream sharing message.

With live stream URLs, you can watch multiple Douyin live streams on your computer.

## Install

```
go get -v -u github.com/caiguanhao/dylive
```

## Usage

In any live room, click Share and select Copy Link (复制链接).

<img src="https://user-images.githubusercontent.com/1284703/121233565-554aa580-c8c5-11eb-97bf-28f25d96057c.jpg" width="400" />

Paste the link to dylive, for example, on Mac:

```
pbpaste | dylive | pbcopy
```

And you will get a link looks like:

```
http://pull-hls-f11.douyincdn.com/third/stream-000000000000000000.m3u8
```

Open this link in video player like QuickTime Player or VLC and you will watch the HD live on your computer.

<img src="https://user-images.githubusercontent.com/1284703/121235401-6c8a9280-c8c7-11eb-947a-6d3d0476ad2b.png" width="400" />

Alternatively, you can run `dylive` and type the URL or just the ID in the URL (`https://v.douyin.com/(id)/`).

After entering URL or ID, press Ctrl-Z and Enter (Windows) or Ctrl-D (Mac or Linux) to finish.

Also works on Windows:

<img src="https://user-images.githubusercontent.com/1284703/121242020-a6ab6280-c8ce-11eb-8db6-bc4385ffc017.png" width="400" />
