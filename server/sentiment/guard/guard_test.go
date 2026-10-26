package guard

import (
	"testing"

	"nvs-server/sentiment/lexicon"
	"nvs-server/sentiment/topic"
)

// opts 生产默认口径：中高以上才拦，中以上标注。
func opts() Options {
	return Options{BlockRisk: topic.RiskMidHigh, FlagRisk: topic.RiskMid, FloodBlock: true}
}

func TestConditionalBlockByTopic(t *testing.T) {
	terms := lexicon.Baseline

	// 纯黑话（乐子/典）在亚文化话题下只标注：这类词在二次元评论区是日常用语，
	// 升级成拦截等于把整个评论区删光
	sub := topic.Assess([]string{"二次元", "同人"}, "")
	d1 := Evaluate("这波操作真是典中典，乐子人狂喜", sub, opts(), terms)
	if d1.Action == ActionBlock {
		t.Fatalf("纯黑话不该升级为拦截，得到 %s（reason=%s）", d1.Action, d1.Reason)
	}
	if d1.Action != ActionFlag {
		t.Fatalf("亚文化话题下应标注，得到 %s", d1.Action)
	}

	// 同样属于亚文化话题，但词本身是辱骂（severity 3）→ 拦截
	d3 := Evaluate("画质差就说画质差，骂人傻逼算什么本事", sub, opts(), terms)
	if d3.Action != ActionBlock {
		t.Fatalf("亚文化话题下辱骂应拦截，得到 %s（reason=%s）", d3.Action, d3.Reason)
	}

	// 辱骂放在无关话题（低风险）里只留证、不拦
	low := topic.Assess([]string{"玄幻小说"}, "")
	d2 := Evaluate("画质差就说画质差，骂人傻逼算什么本事", low, opts(), terms)
	if d2.Action == ActionBlock {
		t.Fatalf("低风险话题不该拦，得到 %s（reason=%s）", d2.Action, d2.Reason)
	}
}

func TestTopicCouplingOnlyEscalatesInMatchingGroup(t *testing.T) {
	terms := lexicon.Baseline
	// 「女拳」绑定性别议题组：在资本批判话题下不该升级为拦截
	capital := topic.Assess([]string{"资本家", "996"}, "")
	d := Evaluate("某些女拳又来带节奏了", capital, opts(), terms)
	if d.Action == ActionBlock {
		t.Fatalf("话题不匹配却拦截了（说明耦合失效）：%s / %s", d.Action, d.Reason)
	}

	// 在性别议题话题下应拦
	gender := topic.Assess([]string{"性别议题"}, "")
	d2 := Evaluate("某些女拳又来带节奏了", gender, opts(), terms)
	if d2.Action != ActionBlock {
		t.Fatalf("性别话题下应拦截，得到 %s（reason=%s）", d2.Action, d2.Reason)
	}
}

func TestForceBlockIgnoresTopic(t *testing.T) {
	terms := []lexicon.Term{
		{Word: "测试强词", Cat: lexicon.CatAbuse, Severity: 3, ForceBlock: true},
	}
	low := topic.Assess([]string{"玄幻"}, "")
	d := Evaluate("这里有测试强词出现", low, opts(), terms)
	if d.Action != ActionBlock {
		t.Fatalf("强制词应无视话题拦截，得到 %s", d.Action)
	}
}

func TestFloodBlock(t *testing.T) {
	low := topic.Assess([]string{"玄幻"}, "")
	// 大量 @ + 连续感叹号：典型出征喊话
	d := Evaluate("@张三 @李四 @王五 @赵六 @钱七 都来看！！！！！！！", low, opts(), lexicon.Baseline)
	if d.Action != ActionBlock {
		t.Fatalf("刷屏出征应拦截，得到 %s（flood=%+v）", d.Action, d.Flood)
	}
	// 正常长评论不应被判刷屏
	normal := "这一章的节奏把握得不错，尤其是结尾那段留白，读起来很有余味。希望作者保持。"
	d2 := Evaluate(normal, low, opts(), lexicon.Baseline)
	if d2.Action == ActionBlock {
		t.Fatalf("正常评论被拦：%s / %s", d2.Action, d2.Reason)
	}
}

func TestFlagOnlyUnderMidRisk(t *testing.T) {
	terms := lexicon.Baseline
	// 政治批判单独=中，正好达到标注门槛但低于拦截门槛
	pol := topic.Assess([]string{"时政"}, "")
	d := Evaluate("这帮人真是废物", pol, opts(), terms)
	if d.Action != ActionFlag {
		t.Fatalf("中风险话题下应只标注，得到 %s（reason=%s）", d.Action, d.Reason)
	}
	if len(d.Flagged) == 0 {
		t.Fatalf("标注应记录命中词")
	}
}

func TestAllowWhenNothingFound(t *testing.T) {
	pol := topic.Assess([]string{"时政"}, "")
	d := Evaluate("这个政策的具体条款我读完了，觉得还有改进空间。", pol, opts(), lexicon.Baseline)
	if d.Action != ActionAllow {
		t.Fatalf("干净评论应为 allow，得到 %s（%s）", d.Action, d.Reason)
	}
}

func TestFoldThreshold(t *testing.T) {
	if FoldThreshold("短文本", 400) {
		t.Fatalf("短文本不该折叠")
	}
	long := ""
	for i := 0; i < 500; i++ {
		long += "字"
	}
	if !FoldThreshold(long, 400) {
		t.Fatalf("长文本应折叠")
	}
	if FoldThreshold(long, 0) {
		t.Fatalf("阈值 0 表示不折叠")
	}
}
