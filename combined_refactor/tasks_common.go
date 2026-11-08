package main

import (
	"context"
	"fmt"
	"math"
	"runtime/debug"
	"sync"
	"time"
)

const progressEmitInterval = 250 * time.Millisecond

func runBoundedWorkers(ctx context.Context, total, maxWorkers, progressEvery int, onProgress func(current, total, etaSeconds int), work func(idx int)) bool {
	wasCanceled, _ := runBoundedWorkersUntil(ctx, total, maxWorkers, progressEvery, onProgress, nil, work)
	return wasCanceled
}

// runBoundedWorkersUntil 在调度新任务前检查 shouldStop；返回时仍会等待已在途任务结束。
// onProgress 的 etaSeconds 为按窗口速率估算的剩余秒数，-1 表示样本不足无法估算。
func runBoundedWorkersUntil(ctx context.Context, total, maxWorkers, progressEvery int, onProgress func(current, total, etaSeconds int), shouldStop func() bool, work func(idx int)) (bool, bool) {
	if total <= 0 {
		return false, false
	}
	if maxWorkers <= 0 {
		maxWorkers = 1
	}

	var wg sync.WaitGroup
	slots := make(chan struct{}, maxWorkers)
	var count int
	var countMutex sync.Mutex
	var lastEmit time.Time
	stoppedEarly := false
	eta := newEtaEstimator()

	for i := 0; i < total; i++ {
		if shouldStop != nil && shouldStop() {
			stoppedEarly = true
			break
		}
		select {
		case <-ctx.Done():
			return true, stoppedEarly
		case slots <- struct{}{}:
		}

		wg.Add(1)
		go func(idx int) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("worker panic (idx=%d): %v\n%s\n", idx, r, debug.Stack())
				}
				<-slots
				wg.Done()

				countMutex.Lock()
				count++
				current := count
				shouldEmit := false
				now := time.Now()
				if current == total {
					shouldEmit = true
				} else if progressEvery > 1 && current%progressEvery == 0 {
					shouldEmit = true
				} else if now.Sub(lastEmit) >= progressEmitInterval {
					shouldEmit = true
				}
				if shouldEmit {
					lastEmit = now
				}
				etaSeconds := -1
				if shouldEmit {
					etaSeconds = eta.etaSeconds(current, total, now)
				}
				countMutex.Unlock()
				eta.record(now)

				if onProgress != nil && shouldEmit {
					func() {
						defer func() {
							if r := recover(); r != nil {
								fmt.Printf("onProgress panic: %v\n%s\n", r, debug.Stack())
							}
						}()
						onProgress(current, total, etaSeconds)
					}()
				}
			}()

			select {
			case <-ctx.Done():
				return
			default:
			}

			// 网络拥塞时放慢取任务节奏（探测连续失败才触发，恢复后自动停止）
			if delay := currentNetHealthThrottle(); delay > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
				}
			}

			work(idx)
		}(i)
	}

	wg.Wait()
	return false, stoppedEarly
}

func reportNSBProgress(session *appSession, phase string, current, total int, text string) {
	reportNSBProgressWithETA(session, phase, current, total, -1, text)
}

// speedTestWithCongestionRecheck 包装一次测速尝试：首次失败且网络健康探针处于拥塞
// 判定状态时，等待 300–800ms 抖动后复核一次（预算内），以复核结果为最终结果。
// 学习 subs-check-pro 的拥塞复核思路（docs/analyses/subs-check-pro-learnings.md 调查②）。
func speedTestWithCongestionRecheck(ctx context.Context, attempt func() (float64, string)) (float64, string) {
	speed, speedErr := attempt()
	if speedErr != "" && congestionRecheck(ctx) {
		return attempt()
	}
	return speed, speedErr
}

// beginSpeedStage 为一轮批量测速建立任务级治理：总带宽令牌桶、网络健康探针
// （编排管线内已有探针时借用，独立测速任务自起自停）、同网段乱序（只改被测顺序）。
// deriveConcurrency 为 true 时按 L/r 收敛并发（总带宽 ÷ 合格阈值，仅非标测速使用；
// 官方测速保持串行节奏，见 brief 决定 D7）。返回收敛后的并发数与清理函数。
func beginSpeedStage(session *appSession, total, threads int, speedMin float64, bandwidth int, shuffle func(), deriveConcurrency bool, stageLabel string) (int, func()) {
	stopNetHealth := ensureNetHealth(total)
	stopBucket := setSpeedTestBandwidthMB(float64(bandwidth))
	if deriveConcurrency && threads > 0 {
		if resolved := resolveSpeedThreads(threads, speedMin, float64(bandwidth)); resolved != threads {
			session.sendWSMessage("log", fmt.Sprintf("%s测速并发按总带宽收敛：%d → %d（总带宽 %dMB/s，合格阈值 %.2fMB/s）", stageLabel, threads, resolved, bandwidth, speedMin))
			threads = resolved
		}
	}
	if shuffle != nil {
		shuffle()
	}
	return threads, func() {
		stopBucket()
		stopNetHealth()
	}
}

// reportNSBProgressWithETA 在 nsb_progress 消息上附带可选 etaSeconds（-1 表示省略字段）
func reportNSBProgressWithETA(session *appSession, phase string, current, total, etaSeconds int, text string) {
	percentage := 0.0
	if total > 0 {
		percentage = float64(current) / float64(total) * 100
	}
	payload := map[string]interface{}{
		"phase":   phase,
		"current": current,
		"total":   total,
		"percent": fmt.Sprintf("%.2f", percentage),
		"text":    text,
	}
	if etaSeconds >= 0 {
		payload["etaSeconds"] = etaSeconds
	}
	session.sendWSMessage("nsb_progress", payload)
}

// resolveScanThreads 解析扫描并发数：显式正数原样生效；
// 0 表示按候选规模自适应（对数衰减，8-400 上下限），
// 学习 subs-check-pro 的衰减自适应思路（docs/analyses/subs-check-pro-learnings.md 建议⑨）。
func resolveScanThreads(requested, candidates int) int {
	if requested > 0 {
		return requested
	}
	if candidates <= 0 {
		return 8
	}
	scale := 80.0 * math.Log(1+float64(candidates)/100.0)
	resolved := int(scale + 0.5)
	if resolved < 8 {
		resolved = 8
	}
	if resolved > 400 {
		resolved = 400
	}
	return resolved
}
