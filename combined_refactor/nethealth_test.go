package main

import (
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
