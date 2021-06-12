package douyinapi

import (
	"testing"
)

func TestGetIdFromUrl(t *testing.T) {
	text := `#在抖音，记录美好生活#【红警直播舞虾】正在直播，` +
		`来和我一起支持Ta吧。复制下方链接，打开【抖音】，` +
		`直接观看直播！ https://v.douyin.com/e9oSECC/`
	userId, roomId, _ := GetIdFromUrl(GetPageUrl(text))
	if roomId != 6972728684293999374 {
		t.Error("wrong room id:", roomId)
	}
	text = `快来加入抖音，让你发现最有趣的我！ ` +
		`https://v.douyin.com/e9oPjy7/`
	userId, roomId, _ = GetIdFromUrl(GetPageUrl(text))
	if userId != 94792729333 {
		t.Error("wrong user id", userId)
	}
	user, _ := GetUserInfo(66178590413, userId)
	if user == nil {
		t.Error("user should not be nil")
	} else {
		if user.Name != "红警直播舞虾" {
			t.Error("wrong user name:", user.Name)
		}
	}
}
