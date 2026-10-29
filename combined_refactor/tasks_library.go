package main

import (
	"context"
	"fmt"
	"sync"
)

func runDCLibraryScan(ctx context.Context, session *appSession, ipType int, scanMaxThreads int, port int, delay int, scanMode string) {
	ipList, err := officialCandidatePool(ipType)
	if err != nil {
		session.sendWSMessage("error", err.Error())
		return
	}
	if len(ipList) == 0 {
		session.sendWSMessage("error", "候选 IP 列表为空，无法更新数据中心库")
		return
	}
	ipType = normalizeDCLibraryIPType(ipType)
	session.sendWSMessage("log", fmt.Sprintf("开始扫描数据中心库（IPv%d）：%d 条记录", ipType, len(ipList)))

	scanner := officialScanFunc(scanOfficialIP)
	if scanMode == scanModeHTTPing {
		scanner = scanOfficialHTTP
	}

	var results []ScanResult
	var resultMutex sync.Mutex
	total := len(ipList)
	session.sendWSMessage("dc_library_progress", map[string]interface{}{"current": 0, "total": total})
	wasCanceled := runBoundedWorkers(ctx, total, scanMaxThreads, 10, func(current, total int) {
		session.sendWSMessage("dc_library_progress", map[string]interface{}{"current": current, "total": total})
	}, func(idx int) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		res, _, _ := scanner(ctx, ipList[idx], port, delay)
		if res == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		resultMutex.Lock()
		results = append(results, *res)
		resultMutex.Unlock()
	})

	if wasCanceled || ctx.Err() != nil {
		session.sendWSMessage("log", "数据中心库更新已终止，保留原有库数据")
		return
	}
	if len(results) == 0 {
		session.sendWSMessage("error", "数据中心库扫描完成，但没有发现可用 IP，已保留原有库数据")
		return
	}
	if err := saveDCLibrary(ipType, results); err != nil {
		session.sendWSMessage("error", "保存数据中心库失败: "+err.Error())
		return
	}
	status := dcLibraryStatusFor(ipType)
	session.sendWSMessage("dc_library_status", dcLibrariesPayload())
	session.sendWSMessage("log", fmt.Sprintf("数据中心库已更新（IPv%d）：%d 个数据中心，%d 个 IP", ipType, status.DCCount, status.IPCount))
}
