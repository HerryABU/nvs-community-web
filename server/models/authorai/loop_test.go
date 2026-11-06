package authorai

import "testing"

func TestParseLoops(t *testing.T) {
	// 嵌套循环 + 事件
	text := "" +
		"1980 故事开始\n" +
		"[LOOP:outer] 外层循环开始\n" +
		"  主角回到1987\n" +
		"  [LOOP:inner] 内层循环\n" +
		"    发现线索\n" +
		"  [LOOPEND:inner] 内层结束\n" +
		"[LOOPEND:outer] 外层结束\n" +
		"2040 结局\n"
	res := ParseLoops(text)
	if !res.HasLoop {
		t.Fatal("expected has_loop true")
	}
	if len(res.Loops) != 2 {
		t.Fatalf("expected 2 loops, got %d", len(res.Loops))
	}
	if res.Loops[0].LoopID != "outer" || res.Loops[0].Depth != 1 {
		t.Fatalf("outer loop wrong: %+v", res.Loops[0])
	}
	if res.Loops[1].LoopID != "inner" || res.Loops[1].ParentLoopID != "outer" || res.Loops[1].Depth != 2 {
		t.Fatalf("inner loop wrong: %+v", res.Loops[1])
	}
	if len(res.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", res.Errors)
	}
	// 事件应携带循环栈
	if res.Events[1].LoopID != "outer" {
		t.Fatalf("event loop id wrong: %+v", res.Events[1])
	}

	// 不平衡：只开不关
	bad := "[LOOP:a] open\n[LOOP:b] open\n[LOOPEND:a] close a but not b"
	rb := ParseLoops(bad)
	if len(rb.Errors) == 0 {
		t.Fatal("expected error for unclosed loop")
	}

	// 顺序错配
	bad2 := "[LOOP:a] a\n[LOOP:b] b\n[LOOPEND:a] wrong\n"
	rb2 := ParseLoops(bad2)
	if len(rb2.Errors) == 0 {
		t.Fatal("expected error for mismatched close order")
	}

	// 重复 ID
	bad3 := "[LOOP:x] x\n[LOOPEND:x]\n[LOOP:x] again\n"
	rb3 := ParseLoops(bad3)
	if len(rb3.Errors) == 0 {
		t.Fatal("expected error for duplicate loop id")
	}

	// 空文本
	re := ParseLoops("")
	if re.HasLoop || len(re.Errors) != 0 {
		t.Fatal("empty text should yield no loop, no errors")
	}

	// 无标识形式 [LOOP] / [/LOOP] 也应被解析（自动编号）
	noID := "[LOOP]\n  主角回到1987\n  [LOOP]\n    内层\n  [/LOOP]\n[/LOOP]\n结局\n"
	rn := ParseLoops(noID)
	if !rn.HasLoop || len(rn.Loops) != 2 {
		t.Fatalf("no-id form: expected 2 loops, got %d (hasLoop=%v)", len(rn.Loops), rn.HasLoop)
	}
	if len(rn.Errors) != 0 {
		t.Fatalf("no-id form should have no errors, got %v", rn.Errors)
	}

	// 混合形式：[LOOP:a] 用 [/LOOP] 关闭
	mixed := "[LOOP:a]\n  x\n[/LOOP]\n"
	rm := ParseLoops(mixed)
	if !rm.HasLoop || len(rm.Errors) != 0 {
		t.Fatalf("mixed form should parse cleanly, got loops=%d errors=%v", len(rm.Loops), rm.Errors)
	}
}

func TestParseLoopsAnchorsAndReturns(t *testing.T) {
	// 锚点 + 回溯语句：纯时间循环叙事（无 [LOOP] 块）
	text := "" +
		"[A] 1990 世界线A开启\n" +
		"2000 事件一\n" +
		"2005 事件二\n" +
		"returnA 回到世界线A起点重来\n" +
		"2010 事件三（已重置）\n"
	res := ParseLoops(text)
	if !res.HasReturn {
		t.Fatal("expected has_return true")
	}
	if len(res.Anchors) != 1 || res.Anchors[0].Name != "A" {
		t.Fatalf("expected 1 anchor A, got %+v", res.Anchors)
	}
	// 找到 returnA 事件
	var retEv *TimelineEvent
	for i := range res.Events {
		if res.Events[i].IsReturn {
			retEv = &res.Events[i]
		}
	}
	if retEv == nil {
		t.Fatal("expected a return event")
	}
	if retEv.ReturnTo != "A" {
		t.Fatalf("return target wrong: %q", retEv.ReturnTo)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", res.Errors)
	}
	// 锚点解析到事件索引 0（其后第一个事件），回环目标解析到该索引
	if idx := res.ResolveAnchorEventIndex("A"); idx != 0 {
		t.Fatalf("anchor A should resolve to event index 0, got %d", idx)
	}

	// ret（无标识）回到最近锚点：不报错
	text2 := "[B] 1980 起点\n1990 事件\nret 回溯\n"
	r2 := ParseLoops(text2)
	if !r2.HasReturn || len(r2.Errors) != 0 {
		t.Fatalf("bare ret should parse cleanly, got errors=%v", r2.Errors)
	}
	if r2.Events[len(r2.Events)-1].ReturnTo != "" {
		t.Fatalf("bare ret should have empty ReturnTo, got %q", r2.Events[len(r2.Events)-1].ReturnTo)
	}

	// 回环目标未定义 → 容忍（用户原始输入非格式化，需 AI 洗稿；语义由提示词定义，不做硬报错）
	text3 := "2000 事件\nreturnX 跳到不存在的锚点\n"
	r3 := ParseLoops(text3)
	if !r3.HasReturn {
		t.Fatal("expected has_return true even for undefined target")
	}
	if len(r3.Errors) != 0 {
		t.Fatalf("undefined return target should NOT be a hard error, got %v", r3.Errors)
	}
	if idx := r3.ResolveAnchorEventIndex("X"); idx != -2 {
		t.Fatalf("undefined target should resolve to -2, got %d", idx)
	}

	// 锚点重复定义 → 报错
	text4 := "[A] 锚点一\n[A] 锚点二\n"
	r4 := ParseLoops(text4)
	if len(r4.Errors) == 0 {
		t.Fatal("expected error for duplicate anchor")
	}

	// 回环目标可指向 [LOOP:id] 开启点（循环隐式锚点）
	text5 := "[LOOP:day] 一天开始\n  morning 事件\n  returnday 回到当天开始\n[/LOOP]\n"
	r5 := ParseLoops(text5)
	if len(r5.Errors) != 0 {
		t.Fatalf("return targeting loop open should be valid, got %v", r5.Errors)
	}
	if idx := r5.ResolveAnchorEventIndex("day"); idx != 0 {
		t.Fatalf("loop day open should resolve to event index 0, got %d", idx)
	}

	// [A] 不被误判为 [LOOP]；[LOOP] 不被误判为锚点
	text6 := "[LOOP]\n  x\n[/LOOP]\n[A] 锚点\nret 回最近\n"
	r6 := ParseLoops(text6)
	if !r6.HasLoop || len(r6.Anchors) != 1 || r6.Anchors[0].Name != "A" {
		t.Fatalf("loop/anchor classification wrong: loops=%d anchors=%+v", len(r6.Loops), r6.Anchors)
	}
	if len(r6.Errors) != 0 {
		t.Fatalf("mixed loop+anchor should parse cleanly, got %v", r6.Errors)
	}
}
