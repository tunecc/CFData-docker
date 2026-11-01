package main

import "testing"

func TestResolveScanThreadsExplicit(t *testing.T) {
	if got := resolveScanThreads(100, 50000); got != 100 {
		t.Fatalf("显式正数应原样生效，得到 %d", got)
	}
	if got := resolveScanThreads(1, 50000); got != 1 {
		t.Fatalf("显式 1 应生效，得到 %d", got)
	}
}

func TestResolveScanThreadsAutoBounds(t *testing.T) {
	// 小池触底 8
	if got := resolveScanThreads(0, 10); got != 8 {
		t.Fatalf("小池应触底 8，得到 %d", got)
	}
	// 大池触顶 400
	if got := resolveScanThreads(0, 1000000); got != 400 {
		t.Fatalf("大池应触顶 400，得到 %d", got)
	}
	// 候选为 0 时按最小并发兜底
	if got := resolveScanThreads(0, 0); got != 8 {
		t.Fatalf("空候选池应兜底 8，得到 %d", got)
	}
}

func TestResolveScanThreadsAutoScalesMonotonically(t *testing.T) {
	prev := 0
	for _, candidates := range []int{50, 100, 500, 1000, 5000, 10000, 50000, 100000} {
		got := resolveScanThreads(0, candidates)
		if got < 8 || got > 400 {
			t.Fatalf("候选 %d 的自适应值 %d 越界", candidates, got)
		}
		if got < prev {
			t.Fatalf("自适应值应随候选规模单调不减：%d -> %d", prev, got)
		}
		prev = got
	}
}

func TestPrepFunctionsPreserveZeroThreads(t *testing.T) {
	// 回归：0=自动必须穿透 prep 层到达扫描入口的 resolveScanThreads（验收 A7）
	official := cliConfig{ipType: 4, threads: 0, port: 443}
	if err := prepareOfficialCLIConfig(&official); err != nil {
		t.Fatalf("prep 失败: %v", err)
	}
	if official.threads != 0 {
		t.Fatalf("官方 prep 应保留 0=自动，得到 %d", official.threads)
	}
	nsb := cliConfig{ipType: 4, threads: 0, port: 443, file: "ips.txt", resultLimit: 100, nsbFallbackPort: 443}
	if err := prepareNSBCLIConfig(&nsb); err != nil {
		t.Fatalf("prep 失败: %v", err)
	}
	if nsb.threads != 0 {
		t.Fatalf("非标 prep 应保留 0=自动，得到 %d", nsb.threads)
	}
	// 负数回落默认 100
	negative := cliConfig{ipType: 4, threads: -5, port: 443}
	if err := prepareOfficialCLIConfig(&negative); err != nil {
		t.Fatalf("prep 失败: %v", err)
	}
	if negative.threads != 100 {
		t.Fatalf("负数应回落 100，得到 %d", negative.threads)
	}
}
