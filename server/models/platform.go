package models

import "time"

// Report 举报
type Report struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	ReporterID uint       `gorm:"not null;index" json:"reporter_id"`
	TargetType string     `gorm:"size:16;not null;index:idx_reports_target,priority:1" json:"target_type"` // novel/comment/thread/post/blog/forum/chapter/user/userframe/userhtml
	TargetID   uint       `gorm:"not null;index:idx_reports_target,priority:2" json:"target_id"`
	ReasonCode string     `gorm:"size:32;default:''" json:"reason_code"` // 预置理由编码
	Reason     string     `gorm:"size:64;not null" json:"reason"`        // 理由展示文本
	Detail     string     `gorm:"type:text" json:"detail"`
	Status     string     `gorm:"size:16;default:pending;index" json:"status"` // pending/accepted/rejected
	HandlerID  *uint      `json:"handler_id"`
	Verdict    string     `gorm:"type:text" json:"verdict"`
	ResolvedAt *time.Time `json:"resolved_at"`
	CreatedAt  time.Time  `json:"created_at"`
	Reporter   *User      `gorm:"foreignKey:ReporterID" json:"reporter,omitempty"`
}

func (Report) TableName() string { return "reports" }

// ReportReason 预置举报理由
type ReportReason struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Desc  string `json:"desc"`
}

// PredefinedReportReasons 平台预置举报理由（供前端选择）
func PredefinedReportReasons() []ReportReason {
	return []ReportReason{
		{Code: "spam", Label: "垃圾广告", Desc: "广告、刷屏、外链推广等无关内容"},
		{Code: "abuse", Label: "辱骂攻击", Desc: "人身攻击、侮辱谩骂、网络暴力"},
		{Code: "porn", Label: "色情低俗", Desc: "色情、淫秽或低俗内容"},
		{Code: "violence", Label: "暴力血腥", Desc: "宣扬暴力、血腥、恐怖内容"},
		{Code: "illegal", Label: "违法违规", Desc: "违反法律法规或危害国家安全的内容"},
		{Code: "politics", Label: "涉政敏感", Desc: "涉及政治敏感、危害社会稳定等红线内容"},
		{Code: "infringement", Label: "抄袭侵权", Desc: "抄袭、盗用他人原创作品或版权内容"},
		{Code: "misleading", Label: "虚假误导", Desc: "虚假信息、误导性陈述、谣言"},
		{Code: "privacy", Label: "泄露隐私", Desc: "泄露他人姓名、住址、联系方式等个人隐私"},
		{Code: "threat", Label: "人身威胁", Desc: "威胁恐吓、骚扰、人肉搜索等行为"},
		{Code: "gambling", Label: "赌博诈骗", Desc: "赌博、诈骗、传销、非法集资等违法内容"},
		{Code: "minors", Label: "危害未成年人", Desc: "危害未成年人身心健康的有害内容"},
		{Code: "baiting", Label: "恶意引战", Desc: "跨区引战、挑动对立、恶意炒作"},
		{Code: "malware", Label: "恶意代码", Desc: "含病毒、木马、钓鱼或恶意脚本的 HTML/扩展"},
		{Code: "other", Label: "其他", Desc: "其他需要说明的问题"},
	}
}

// LookupReportReason 根据编码查找预置理由，未命中返回 false
func LookupReportReason(code string) (ReportReason, bool) {
	for _, r := range PredefinedReportReasons() {
		if r.Code == code {
			return r, true
		}
	}
	return ReportReason{}, false
}

// WithdrawalRequest 提现申请
type WithdrawalRequest struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Amount    float64   `gorm:"not null" json:"amount"`
	Method    string    `gorm:"size:32" json:"method"`
	Account   string    `gorm:"size:128" json:"account"`
	Status    string    `gorm:"size:16;default:pending" json:"status"` // pending/approved/paid/rejected
	CreatedAt time.Time `json:"created_at"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (WithdrawalRequest) TableName() string { return "withdrawal_requests" }

// EarningsRecord 收益记录
type EarningsRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	NovelID   *uint     `json:"novel_id"`
	Amount    float64   `gorm:"not null" json:"amount"`
	Type      string    `gorm:"size:32;not null" json:"type"` // tip/subscription
	CreatedAt time.Time `json:"created_at"`
}

func (EarningsRecord) TableName() string { return "earnings_records" }

// BlacklistIP 黑名单 IP
type BlacklistIP struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	IP        string    `gorm:"size:45;uniqueIndex;not null" json:"ip"`
	Reason    string    `gorm:"size:256" json:"reason"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (BlacklistIP) TableName() string { return "blacklist_ips" }
