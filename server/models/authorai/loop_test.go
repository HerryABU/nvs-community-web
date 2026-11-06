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
