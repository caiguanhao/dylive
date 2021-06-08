package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
)

var (
	errorNoUrl = errors.New("No URL found")
)

const (
	scriptOpen  = "<script>window.__INIT_PROPS__ = "
	scriptClose = "</script>"
)

func main() {
	text, _ := ioutil.ReadAll(os.Stdin)
	re := regexp.MustCompile(`https?://v\.douyin\.com/([A-Za-z0-9]+)/`)
	url := re.FindString(string(text))
	if url == "" {
		re = regexp.MustCompile(`[A-Za-z0-9]{7}`)
		url = re.FindString(string(text))
		if url != "" {
			url = "https://v.douyin.com/" + url + "/"
		}
	}
	if err := get(url); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func get(url string) error {
	if url == "" {
		return errorNoUrl
	}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1")
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	url = getUrl(string(b))
	if url == "" {
		return errorNoUrl
	}
	fmt.Print(strings.TrimSpace(url)) // print url to stdout
	return nil
}

func getUrl(html string) string {
	i := strings.Index(html, scriptOpen)
	if i > -1 {
		html = html[i+len(scriptOpen):]
	}
	i = strings.Index(html, scriptClose)
	html = html[:i]
	var obj map[string]map[string]map[string]map[string]map[string]string
	json.Unmarshal([]byte(html), &obj)
	var out string
	for _, v := range obj {
		urlMap := v["room"]["stream_url"]["hls_pull_url_map"]
		if url := urlMap["FULL_HD1"]; url != "" {
			out = url
		} else if url := urlMap["HD1"]; url != "" {
			out = url
		}
		for key, url := range urlMap {
			fmt.Fprintln(os.Stderr, key, url)
			if out == "" {
				out = url
			}
		}
	}
	return out
}
