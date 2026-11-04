package collab

import "testing"

// TestCan_OwnerPlatformPrecedence 回归 #63：admin/police 同时是作品主笔时，
// 不能被 Can 里的平台分支剥夺其创作能力（此前 CreateChapter/PublishChapter 对站长自建作品 403）。
// 平台分支的「不获得创作能力」只应作用于「平台角色在他人作品上」的场景。
func TestCan_OwnerPlatformPrecedence(t *testing.T) {
	cases := []struct {
		name string
		a    Actor
		cap  Capability
		want bool
	}{
		{"主笔兼平台角色：可新建章节", Actor{IsOwner: true, IsPlatform: true}, CapWriteChapter, true},
		{"主笔兼平台角色：可直接发布", Actor{IsOwner: true, IsPlatform: true}, CapPublishChapter, true},
		{"主笔兼平台角色：可改他人章节", Actor{IsOwner: true, IsPlatform: true}, CapEditOthers, true},
		{"主笔兼平台角色：可管理协作", Actor{IsOwner: true, IsPlatform: true}, CapManageCollab, true},
		{"主笔兼平台角色：可切换模式", Actor{IsOwner: true, IsPlatform: true}, CapSwitchMode, true},
		{"主笔兼平台角色：可导出", Actor{IsOwner: true, IsPlatform: true}, CapExport, true},
		{"主笔兼平台角色：可看数据", Actor{IsOwner: true, IsPlatform: true}, CapViewStats, true},
		{"主笔兼平台角色：可审核", Actor{IsOwner: true, IsPlatform: true}, CapReviewChapter, true},

		{"平台角色在他人作品：不可新建章节", Actor{IsOwner: false, IsPlatform: true}, CapWriteChapter, false},
		{"平台角色在他人作品：不可直接发布", Actor{IsOwner: false, IsPlatform: true}, CapPublishChapter, false},
		{"平台角色在他人作品：可导出（合规）", Actor{IsOwner: false, IsPlatform: true}, CapExport, true},
		{"平台角色在他人作品：可看数据（合规）", Actor{IsOwner: false, IsPlatform: true}, CapViewStats, true},
		{"平台角色在他人作品：可审核（合规）", Actor{IsOwner: false, IsPlatform: true}, CapReviewChapter, true},

		{"普通主笔：可新建章节", Actor{IsOwner: true, IsPlatform: false}, CapWriteChapter, true},
		{"匿名非成员：不可新建章节", Actor{IsOwner: false, IsPlatform: false}, CapWriteChapter, false},
	}
	for _, c := range cases {
		if got := c.a.Can(c.cap); got != c.want {
			t.Errorf("%s: Can(%q) = %v, want %v", c.name, c.cap, got, c.want)
		}
	}
}
