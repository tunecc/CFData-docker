package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNetHealthWatcherHealthyWhenProbeSucceeds(t *testing.T) {
	w := newNetHealthWatcher(func(target string, timeout time.Duration) error { return nil })
	w.probeOnce()
	if !w.isHealthy() {
		t.Fatal("探测成功应判定健康")
	}
	if got := currentNetHealthThrottleFor(w); got != 0 {
		t.Fatalf("健康时不应节流，得到 %v", got)
	}
}

func TestNetHealthWatcherCongestionAfterStreak(t *testing.T) {
	calls := 0
	w := newNetHealthWatcher(func(target string, timeout time.Duration) error {
		calls++
		return errors.New("timeout")
	})
	// 连续失败不足阈值：仍视为健康（不误节流）
	w.probeOnce()
	w.probeOnce()
	if !w.isHealthy() {
		t.Fatal("失败未达阈值不应判定拥塞")
	}
	w.probeOnce()
	if w.isHealthy() {
		t.Fatal("连续 3 轮全失败应判定拥塞")
	}
	if got := currentNetHealthThrottleFor(w); got != netHealthThrottleDelay {
		t.Fatalf("拥塞时应返回节流延迟，得到 %v", got)
	}
	// 恢复：任一成功即回到健康并清空告警状态
	w = newNetHealthWatcher(func(target string, timeout time.Duration) error {
		calls++
		if calls%2 == 0 {
			return nil
		}
		return errors.New("timeout")
	})
	w.probeOnce()
	w.probeOnce()
	w.probeOnce()
	w.probeOnce() // 第 4 轮成功
	if !w.isHealthy() {
		t.Fatal("探测恢复后应判定健康")
	}
	if got := currentNetHealthThrottleFor(w); got != 0 {
		t.Fatalf("恢复后不应节流，得到 %v", got)
	}
}

func TestNetHealthWatcherAnyTargetSuccessIsHealthy(t *testing.T) {
	// 第一个目标失败、第二个成功 → 健康
	w := newNetHealthWatcher(func(target string, timeout time.Duration) error {
		if target == netHealthProbeTargets[0] {
			return errors.New("fail")
		}
		return nil
	})
	w.probeOnce()
	if !w.isHealthy() {
		t.Fatal("任一目标成功即应判定健康")
	}
}

func TestStartScanNetHealthRegistersAndStops(t *testing.T) {
	orig := activeNetHealth.Load()
	stop := startScanNetHealth()
	if activeNetHealth.Load() == nil {
		t.Fatal("启动后应注册活跃探针")
	}
	stop()
	if activeNetHealth.Load() != orig {
		t.Fatal("停止后应恢复原探针状态")
	}
}

func currentNetHealthThrottleFor(w *netHealthWatcher) time.Duration {
	saved := activeNetHealth.Load()
	activeNetHealth.Store(w)
	defer func() { activeNetHealth.Store(saved) }()
	return currentNetHealthThrottle()
}

// A2：拥塞期复核——仅在拥塞判定状态且预算内触发，抖动 300–800ms，预算耗尽即停。
func TestCongestionRecheckBudgetAndJitter(t *testing.T) {
	w := newNetHealthWatcher(func(target string, timeout time.Duration) error { return errors.New("down") })
	prev := activeNetHealth.Swap(w)
	defer activeNetHealth.Store(prev)

	// 默认抖动落在 300–800ms 区间（含边界）
	for range 64 {
		if d := defaultRecheckJitter(); d < netHealthRecheckBase || d > netHealthRecheckBase+netHealthRecheckSpan {
			t.Fatalf("抖动越界: %v", d)
		}
	}

	// 未判定拥塞（失败未达阈值）时不复核、不消耗预算
	w.probeOnce()
	w.probeOnce()
	w.initRecheckBudget(30) // max(20, 30/20) = 20
	if congestionRecheck(context.Background()) {
		t.Fatal("未判定拥塞时不应复核")
	}
	if got := w.recheckBudget.Load(); got != 20 {
		t.Fatalf("未拥塞时预算不应消耗: %d", got)
	}

	// 达到阈值判定拥塞
	w.probeOnce()
	if w.isHealthy() {
		t.Fatal("连续 3 轮失败应判定拥塞")
	}

	// 固定抖动消除时长随机性
	netHealthRecheckJitter = func() time.Duration { return time.Millisecond }
	defer func() { netHealthRecheckJitter = defaultRecheckJitter }()

	for i := 0; i < 20; i++ {
		if !congestionRecheck(context.Background()) {
			t.Fatalf("预算内第 %d 次复核应获批", i+1)
		}
	}
	if congestionRecheck(context.Background()) {
		t.Fatal("预算耗尽后不应再复核")
	}
}

// A2：复核包装——首次失败、复核成功 → 采用第二次结果；复核也失败 → 以失败为最终结果。
func TestSpeedTestWithCongestionRecheck(t *testing.T) {
	w := newNetHealthWatcher(func(target string, timeout time.Duration) error { return errors.New("down") })
	prev := activeNetHealth.Swap(w)
	defer activeNetHealth.Store(prev)
	for range netHealthCongestionThreshold {
		w.probeOnce()
	}
	w.initRecheckBudget(20)
	netHealthRecheckJitter = func() time.Duration { return time.Millisecond }
	defer func() { netHealthRecheckJitter = defaultRecheckJitter }()

	calls := 0
	speed, speedErr := speedTestWithCongestionRecheck(context.Background(), func() (float64, string) {
		calls++
		if calls == 1 {
			return 0, "测速请求失败: i/o timeout"
		}
		return 5.5, ""
	})
	if speedErr != "" || speed != 5.5 {
		t.Fatalf("复核成功应采用第二次结果: speed=%v err=%q", speed, speedErr)
	}
	if calls != 2 {
		t.Fatalf("应恰好尝试两次，实际 %d", calls)
	}

	// 复核也失败：以失败收场，不再第三轮
	calls = 0
	_, speedErr = speedTestWithCongestionRecheck(context.Background(), func() (float64, string) {
		calls++
		return 0, "测速请求失败: refused"
	})
	if speedErr == "" || calls != 2 {
		t.Fatalf("复核失败应以失败收场: calls=%d err=%q", calls, speedErr)
	}
}

// ensureNetHealth：无探针时自起自停；已有探针时借用（stop 为空操作，原探针保留）。
func TestEnsureNetHealthBorrowsActiveProbe(t *testing.T) {
	existing := newNetHealthWatcher(func(string, time.Duration) error { return nil })
	prev := activeNetHealth.Swap(existing)
	defer activeNetHealth.Store(prev)

	stop := ensureNetHealth(100)
	stop()
	if activeNetHealth.Load() != existing {
		t.Fatal("借用语义下不应替换或停止已有探针")
	}

	// 无探针：自起自停
	activeNetHealth.Store(nil)
	stop = ensureNetHealth(100)
	if activeNetHealth.Load() == nil {
		t.Fatal("无探针时应启动新探针")
	}
	stop()
	if activeNetHealth.Load() != nil {
		t.Fatal("stop 后应清理自起探针")
	}
}
