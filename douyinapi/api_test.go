package douyinapi

import (
	"testing"
)

func TestGetUserIdByName(t *testing.T) {
	user, err := GetUserByName("hongjingzhibo")
	if err != nil {
		t.Fatal(err)
	}
	if user.SecUid != "MS4wLjABAAAAuw4X7CNDvaXlGM7HE-jp2jMtQC9U0lkICEE-Pg8i7AM" {
		t.Error("bad user secuid", user.SecUid)
	}
	if user.NickName != "红警直播舞虾" {
		t.Error("bad user nickname", user.NickName)
	}
}
