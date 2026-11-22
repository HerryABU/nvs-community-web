package selfhost

import "testing"

func TestRuntimeRequestApproveCanChangeVersion(t *testing.T) {
	setupDB(t)
	req, err := CreateRuntimeRequest(2, RuntimeRequest{Language: "java", Version: "17", Note: "项目需要 Java 17"})
	if err != nil {
		t.Fatal(err)
	}
	// 管理员看完代码后把版本改成 21，并登记可执行路径。
	rt, err := ReviewRuntimeRequest(req.ID, 1, true, "java", "21", "C:/tools/jdk21/bin/java.exe", "代码用了 21 特性")
	if err != nil {
		t.Fatalf("审批失败: %v", err)
	}
	if rt == nil || rt.Version != "21" || rt.ExecPath == "" {
		t.Fatalf("运行时未按管理员修改落地: %+v", rt)
	}
	list, _ := ListRuntimeRequests("approved")
	if len(list) != 1 || list[0].FinalVersion != "21" {
		t.Fatalf("申请未记录最终版本: %+v", list)
	}

	req2, _ := CreateRuntimeRequest(2, RuntimeRequest{Language: "go", Version: "1.20", Note: "备用"})
	if _, err := ReviewRuntimeRequest(req2.ID, 1, false, "", "", "", "暂不支持"); err != nil {
		t.Fatal(err)
	}
	list, _ = ListRuntimeRequests("rejected")
	if len(list) != 1 {
		t.Fatalf("驳回记录缺失: %+v", list)
	}
}

func TestScanPathRuntimesIsBounded(t *testing.T) {
	// 不依赖具体机器是否装了运行时，只验证返回结构可控且不会 panic。
	out := ScanPathRuntimes()
	if len(out) > len(runtimeProbes) {
		t.Fatalf("扫描结果超出探针数量: %d", len(out))
	}
	for _, c := range out {
		if c.Language == "" || c.ExecPath == "" {
			t.Errorf("扫描结果字段缺失: %+v", c)
		}
	}
}
