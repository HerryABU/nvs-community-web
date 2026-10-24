package risk

import (
	"testing"
	"time"

	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupRiskDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:risk_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	models.DB = db
	if err := db.AutoMigrate(&models.PlatformConfig{}, &models.RechargeOrder{}, &models.User{}); err != nil {
		t.Fatalf("migrate deps: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Exec("DELETE FROM recharge_orders").Error
	_ = db.Exec("DELETE FROM payment_risk_events").Error
	_ = models.SetPlatformConfig(cfgKey, "")
}

func TestEvaluateCleanUserAllow(t *testing.T) {
	setupRiskDB(t)
	res := Evaluate(EvalInput{
		UserID: 1, OrderNo: "R1", AmountFen: 1000, Channel: "wechat",
		IP: "10.0.0.1", DeviceID: "dev-clean",
		UserCreatedAt: time.Now().Add(-30 * 24 * time.Hour),
	})
	if res.Decision != DecisionAllow {
		t.Fatalf("正常用户应放行: %+v", res)
	}
	if res.Score != 0 {
		t.Fatalf("不应有分数: %d", res.Score)
	}
}

func TestEvaluateRapidOrdersHold(t *testing.T) {
	setupRiskDB(t)
	// 窗口内 5 笔（默认阈值 5）
	for i := 0; i < 5; i++ {
		_ = models.DB.Create(&models.RechargeOrder{
			OrderNo: "R" + string(rune('a'+i)), UserID: 2, Channel: "wechat",
			AmountFen: 100, Status: "pending", CreatedAt: time.Now(),
		}).Error
	}
	res := Evaluate(EvalInput{
		UserID: 2, OrderNo: "R-new", AmountFen: 100, Channel: "wechat",
		IP: "10.0.0.2", DeviceID: "dev-rapid",
		UserCreatedAt: time.Now().Add(-30 * 24 * time.Hour),
	})
	// 单一信号只记录不拦（5 笔也可能是正常多笔充值）；关键是要有分数与原因
	if res.Score < 30 {
		t.Fatalf("高频下单应加分: %+v", res)
	}
	found := false
	for _, r := range res.Reasons {
		if r != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("必须给出可解释的原因")
	}
}

func TestEvaluateNewAccountBigAmount(t *testing.T) {
	setupRiskDB(t)
	res := Evaluate(EvalInput{
		UserID: 3, OrderNo: "R3", AmountFen: 60000, Channel: "alipay",
		IP: "10.0.0.3", DeviceID: "dev-new",
		UserCreatedAt: time.Now().Add(-10 * time.Minute), // 新号
	})
	// 新号大额(+20) + 单笔较大(+10) = 30，低于 hold 60 —— 属于「记录但不拦」
	if res.Score < 20 {
		t.Fatalf("新号大额应加分: %+v", res)
	}
	if res.Decision != DecisionAllow {
		t.Fatalf("单独新号大额不应直接拦: %+v", res)
	}
	// 叠加「高频 + 新号大额」到 60 → hold（不静默放行，也不直接拒）
	for i := 0; i < 5; i++ {
		_ = models.DB.Create(&models.RechargeOrder{
			OrderNo: "H" + string(rune('a'+i)), UserID: 3, Channel: "alipay",
			AmountFen: 100, Status: "pending", CreatedAt: time.Now(),
		}).Error
	}
	res2 := Evaluate(EvalInput{
		UserID: 3, OrderNo: "H-new", AmountFen: 120000, Channel: "alipay",
		IP: "10.0.0.3", DeviceID: "dev-new",
		UserCreatedAt: time.Now().Add(-10 * time.Minute),
	})
	if res2.Decision != DecisionHold {
		t.Fatalf("叠加后应为 hold: %+v", res2)
	}
}

func TestEvaluateBlockThreshold(t *testing.T) {
	setupRiskDB(t)
	// 造一个「蜜罐高风险 IP」+ 高频 + 新号大额，叠加到 block
	_ = models.DB.Exec(`CREATE TABLE IF NOT EXISTS honeypot_sessions (
		id integer primary key, src_ip text, max_risk integer)`).Error
	_ = models.DB.Exec(`INSERT INTO honeypot_sessions (src_ip, max_risk) VALUES ('6.6.6.6', 95)`).Error
	_ = models.DB.Exec(`CREATE TABLE IF NOT EXISTS gate_device_bindings (
		id integer primary key, device_id text, user_id integer, ip text, ua_hash text, source text, created_at datetime)`).Error
	for i := 1; i <= 3; i++ {
		_ = models.DB.Exec(`INSERT INTO gate_device_bindings (device_id, user_id) VALUES ('dev-bad', ?)`, i).Error
	}
	for i := 0; i < 6; i++ {
		_ = models.DB.Create(&models.RechargeOrder{
			OrderNo: "B" + string(rune('a'+i)), UserID: 4, Channel: "wechat",
			AmountFen: 100, Status: "pending", CreatedAt: time.Now(),
		}).Error
	}
	res := Evaluate(EvalInput{
		UserID: 4, OrderNo: "B-new", AmountFen: 60000, Channel: "wechat",
		IP: "6.6.6.6", DeviceID: "dev-bad",
		UserCreatedAt: time.Now().Add(-5 * time.Minute),
	})
	if res.Decision != DecisionBlock {
		t.Fatalf("叠加异常应拦截: %+v", res)
	}
	ev, err := Record(EvalInput{UserID: 4, OrderNo: "B-new", AmountFen: 60000, Channel: "wechat",
		IP: "6.6.6.6", DeviceID: "dev-bad"}, res)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Status != StatusOpen {
		t.Fatalf("block 必须留在待复核队列: %s", ev.Status)
	}
	// 人工放行
	if err := Handle(ev.ID, 99, StatusCleared, "确认为本人操作"); err != nil {
		t.Fatal(err)
	}
	var got Event
	_ = models.DB.Where("id = ?", ev.ID).First(&got).Error
	if got.Status != StatusCleared || got.HandledBy != 99 {
		t.Fatalf("复核状态未落库: %+v", got)
	}
}

func TestRecordAllowAutoCleared(t *testing.T) {
	setupRiskDB(t)
	res := Evaluate(EvalInput{UserID: 9, OrderNo: "R9", AmountFen: 1000,
		UserCreatedAt: time.Now().Add(-90 * 24 * time.Hour)})
	ev, err := Record(EvalInput{UserID: 9, OrderNo: "R9", AmountFen: 1000}, res)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Status != StatusCleared {
		t.Fatalf("allow 应直接归档为已放行: %s", ev.Status)
	}
}
