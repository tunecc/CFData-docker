package main

import (
	"context"
	"fmt"
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

			work(idx)
		}(i)
	}

	wg.Wait()
	return false, stoppedEarly
}

func reportNSBProgress(session *appSession, phase string, current, total int, text string) {
	reportNSBProgressWithETA(session, phase, current, total, -1, text)
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
