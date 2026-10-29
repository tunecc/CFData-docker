package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const dcLibrarySchema = "cfdata.dc-library.v1"

type dcLibraryEntry struct {
	IP       string `json:"ip"`
	Colo     string `json:"colo"`
	LastSeen int64  `json:"lastSeen"`
}

type dcLibraryFile struct {
	Schema    string           `json:"schema"`
	IPType    int              `json:"ipType"`
	UpdatedAt int64            `json:"updatedAt"`
	Entries   []dcLibraryEntry `json:"entries"`
}

type dcLibraryDCStatus struct {
	Colo    string `json:"colo"`
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
	IPCount int    `json:"ipCount"`
}

type dcLibraryStatus struct {
	IPType      int                 `json:"ipType"`
	Exists      bool                `json:"exists"`
	UpdatedAt   int64               `json:"updatedAt"`
	IPCount     int                 `json:"ipCount"`
	DCCount     int                 `json:"dcCount"`
	DataCenters []dcLibraryDCStatus `json:"dataCenters"`
	File        string              `json:"file"`
}

var dcLibraryState = struct {
	sync.RWMutex
	libraries map[int]*dcLibraryFile
}{libraries: map[int]*dcLibraryFile{}}

func dcLibraryFilename(ipType int) string {
	if ipType == 6 {
		return "dc-library-v6.json"
	}
	return "dc-library-v4.json"
}

func normalizeDCLibraryIPType(ipType int) int {
	if ipType == 6 {
		return 6
	}
	return 4
}

func loadDCLibraries() {
	for _, ipType := range []int{4, 6} {
		lib, err := readDCLibraryFile(ipType)
		if err != nil {
			fmt.Printf("数据中心库 %s 加载失败: %v\n", dcLibraryFilename(ipType), err)
			lib = nil
		}
		dcLibraryState.Lock()
		if lib == nil {
			delete(dcLibraryState.libraries, ipType)
		} else {
			dcLibraryState.libraries[ipType] = lib
		}
		dcLibraryState.Unlock()
		if lib != nil {
			fmt.Printf("已加载数据中心库 %s: %d 个 IP\n", dcLibraryFilename(ipType), len(lib.Entries))
		}
	}
}

func readDCLibraryFile(ipType int) (*dcLibraryFile, error) {
	data, err := os.ReadFile(dcLibraryFilename(ipType))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var lib dcLibraryFile
	if err := json.Unmarshal(data, &lib); err != nil {
		return nil, err
	}
	if lib.Schema != dcLibrarySchema {
		return nil, fmt.Errorf("不支持的 schema: %q", lib.Schema)
	}
	lib.IPType = normalizeDCLibraryIPType(ipType)
	entries := make([]dcLibraryEntry, 0, len(lib.Entries))
	for _, entry := range lib.Entries {
		ip := strings.TrimSpace(entry.IP)
		colo := normalizeOfficialTargetDC(entry.Colo)
		parsed := net.ParseIP(ip)
		if parsed == nil || colo == "" || !dcLibraryIPMatchesType(parsed, ipType) {
			continue
		}
		entries = append(entries, dcLibraryEntry{IP: ip, Colo: colo, LastSeen: entry.LastSeen})
	}
	lib.Entries = entries
	return &lib, nil
}

func saveDCLibrary(ipType int, results []ScanResult) error {
	ipType = normalizeDCLibraryIPType(ipType)
	now := time.Now().Unix()
	seen := map[string]dcLibraryEntry{}
	for _, res := range results {
		ip := strings.TrimSpace(res.IP)
		colo := normalizeOfficialTargetDC(res.DataCenter)
		parsed := net.ParseIP(ip)
		if parsed == nil || colo == "" || !dcLibraryIPMatchesType(parsed, ipType) {
			continue
		}
		seen[ip] = dcLibraryEntry{IP: ip, Colo: colo, LastSeen: now}
	}
	if len(seen) == 0 {
		return fmt.Errorf("没有可写入的 IP 记录")
	}
	entries := make([]dcLibraryEntry, 0, len(seen))
	for _, entry := range seen {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Colo != entries[j].Colo {
			return entries[i].Colo < entries[j].Colo
		}
		return entries[i].IP < entries[j].IP
	})
	lib := &dcLibraryFile{
		Schema:    dcLibrarySchema,
		IPType:    ipType,
		UpdatedAt: now,
		Entries:   entries,
	}
	data, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWriteFile(dcLibraryFilename(ipType), data, 0644); err != nil {
		return err
	}
	dcLibraryState.Lock()
	dcLibraryState.libraries[ipType] = lib
	dcLibraryState.Unlock()
	return nil
}

func dcLibraryStatusFor(ipType int) dcLibraryStatus {
	ipType = normalizeDCLibraryIPType(ipType)
	dcLibraryState.RLock()
	lib := dcLibraryState.libraries[ipType]
	dcLibraryState.RUnlock()
	status := dcLibraryStatus{IPType: ipType, File: dcLibraryFilename(ipType), DataCenters: []dcLibraryDCStatus{}}
	if lib == nil {
		return status
	}
	status.Exists = true
	status.UpdatedAt = lib.UpdatedAt
	status.IPCount = len(lib.Entries)
	counts := map[string]int{}
	for _, entry := range lib.Entries {
		counts[entry.Colo]++
	}
	for colo, count := range counts {
		loc := locationMap[colo]
		status.DataCenters = append(status.DataCenters, dcLibraryDCStatus{
			Colo:    colo,
			Country: dcFirstNonEmpty(loc.Country, loc.Cca2),
			Region:  dcFirstNonEmpty(loc.Region_zh, loc.Region),
			City:    dcFirstNonEmpty(loc.City_zh, loc.City),
			IPCount: count,
		})
	}
	sort.Slice(status.DataCenters, func(i, j int) bool {
		if status.DataCenters[i].IPCount != status.DataCenters[j].IPCount {
			return status.DataCenters[i].IPCount > status.DataCenters[j].IPCount
		}
		return status.DataCenters[i].Colo < status.DataCenters[j].Colo
	})
	status.DCCount = len(status.DataCenters)
	return status
}

func dcLibrariesPayload() map[string]dcLibraryStatus {
	return map[string]dcLibraryStatus{
		"4": dcLibraryStatusFor(4),
		"6": dcLibraryStatusFor(6),
	}
}

func libraryIPsForDC(ipType int, dc string) []string {
	ipType = normalizeDCLibraryIPType(ipType)
	dc = normalizeOfficialTargetDC(dc)
	dcLibraryState.RLock()
	lib := dcLibraryState.libraries[ipType]
	dcLibraryState.RUnlock()
	if lib == nil || dc == "" {
		return nil
	}
	ips := make([]string, 0, len(lib.Entries))
	for _, entry := range lib.Entries {
		if entry.Colo == dc {
			ips = append(ips, entry.IP)
		}
	}
	return ips
}

func dcLibraryIPMatchesType(ip net.IP, ipType int) bool {
	if ipType == 6 {
		return ip.To4() == nil
	}
	return ip.To4() != nil
}

func dcFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
