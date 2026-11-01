package main

import (
	"testing"
	"time"
)

func TestEtaEstimatorUnknownBeforeSamples(t *testing.T) {
	e := newEtaEstimator()
	now := time.Unix(1000, 0)
	if got := e.etaSeconds(0, 100, now); got != -1 {
		t.Fatalf("无样本时应返回 -1，得到 %d", got)
	}
	// 仅 1 秒观测不足以估算
	e.record(now)
	if got := e.etaSeconds(1, 100, now); got != -1 {
		t.Fatalf("观测不足 2 秒时应返回 -1，得到 %d", got)
	}
}

func TestEtaEstimatorConvergesOnSteadyRate(t *testing.T) {
	e := newEtaEstimator()
	start := time.Unix(1000, 0)
	// 每秒 10 个，持续 5 秒
	for i := int64(0); i < 5; i++ {
		for j := 0; j < 10; j++ {
			e.record(start.Add(time.Duration(i) * time.Second))
		}
	}
	now := start.Add(4 * time.Second)
	// done=50, total=150：剩余 100，速率 10/s → 约 10 秒
	if got := e.etaSeconds(50, 150, now); got < 9 || got > 11 {
		t.Fatalf("稳态速率下 ETA 应接近 10，得到 %d", got)
	}
}

func TestEtaEstimatorCompleteReturnsZero(t *testing.T) {
	e := newEtaEstimator()
	start := time.Unix(1000, 0)
	e.record(start)
	e.record(start.Add(time.Second))
	if got := e.etaSeconds(100, 100, start.Add(time.Second)); got != 0 {
		t.Fatalf("已全部完成应返回 0，得到 %d", got)
	}
}

func TestEtaEstimatorWindowSlideClearsOldBuckets(t *testing.T) {
	e := newEtaEstimator()
	start := time.Unix(1000, 0)
	// 前 5 秒每秒 100 个，随后停止 70 秒（窗口完全滑出）
	for i := int64(0); i < 5; i++ {
		for j := 0; j < 100; j++ {
			e.record(start.Add(time.Duration(i) * time.Second))
		}
	}
	after := start.Add(70 * time.Second)
	e.record(after)
	// 旧桶已全部滑出，只剩 after 这 1 个样本，观测不足 2 秒 → 未知
	if got := e.etaSeconds(501, 1000, after); got != -1 {
		t.Fatalf("窗口滑空后应返回 -1，得到 %d", got)
	}
}

func TestEtaEstimatorClockRewindRestartsWindow(t *testing.T) {
	e := newEtaEstimator()
	start := time.Unix(2000, 0)
	e.record(start)
	e.record(start.Add(time.Second))
	// 时钟回调：视为新窗口重新开始
	if got := e.etaSeconds(2, 100, start.Add(-10*time.Second)); got != -1 {
		t.Fatalf("时钟倒流应返回 -1，得到 %d", got)
	}
	e.record(start.Add(-10 * time.Second))
	e.record(start.Add(-9 * time.Second))
	// 新窗口 2 个样本跨 2 秒：速率 1/s，剩余 98 → 98 秒
	if got := e.etaSeconds(2, 100, start.Add(-9*time.Second)); got < 97 || got > 99 {
		t.Fatalf("新窗口稳态 ETA 应接近 98，得到 %d", got)
	}
}
