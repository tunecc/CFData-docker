package main

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

const (
	asnDBFile       = "GeoLite2-ASN.mmdb"
	asnDatabaseType = "GeoLite2-ASN"
	asnCacheMaxAge  = 30 * 24 * time.Hour
	asnFetchTimeout = 60 * time.Second
)

// GeoLite2-ASN data is provided by MaxMind under CC BY-SA 4.0.
// Mirrors of the release database: https://github.com/seketiti/GeoLiet2
var asnDBURLs = []string{
	"https://cdn.jsdelivr.net/gh/seketiti/GeoLiet2@release/GeoLite2-ASN.mmdb",
	"https://fastly.jsdelivr.net/gh/seketiti/GeoLiet2@release/GeoLite2-ASN.mmdb",
	"https://gcore.jsdelivr.net/gh/seketiti/GeoLiet2@release/GeoLite2-ASN.mmdb",
	"https://raw.githubusercontent.com/seketiti/GeoLiet2/release/GeoLite2-ASN.mmdb",
	"https://jsd.onmicrosoft.cn/gh/seketiti/GeoLiet2@release/GeoLite2-ASN.mmdb",
}

//go:embed fallback/geolite2-asn.snapshot.mmdb.gz
var embeddedASNGzip []byte

type asnRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

var asnLookupState = struct {
	sync.Mutex
	reader            *maxminddb.Reader
	err               error
	loaded            bool
	refreshGeneration uint64
}{}

func lookupASN(ipText string) (string, string) {
	reader, err := getASNReader()
	if err != nil || reader == nil {
		return "", ""
	}
	ip := net.ParseIP(ipText)
	if ip == nil {
		return "", ""
	}
	var record asnRecord
	if err := reader.Lookup(ip, &record); err != nil {
		return "", ""
	}
	if record.AutonomousSystemNumber == 0 && record.AutonomousSystemOrganization == "" {
		return "", ""
	}
	return fmt.Sprintf("AS%d", record.AutonomousSystemNumber), record.AutonomousSystemOrganization
}

func getASNReader() (*maxminddb.Reader, error) {
	asnLookupState.Lock()
	defer asnLookupState.Unlock()
	if asnLookupState.loaded {
		return asnLookupState.reader, asnLookupState.err
	}
	asnLookupState.loaded = true

	if reader, err := openASNCache(); err == nil {
		asnLookupState.reader = reader
		if asnCacheStale(reader) {
			startASNRefreshLocked()
		}
		return reader, nil
	} else if !os.IsNotExist(err) {
		recordDebugError("asn_cache", err.Error())
		sendLog("本地 ASN 数据库不可用，改用内置离线快照: " + err.Error())
	} else {
		sendLog("本地 ASN 数据库不存在，改用内置离线快照")
	}

	reader, err := openEmbeddedASNReader()
	if err != nil {
		asnLookupState.err = err
		sendLog("加载内置 ASN 数据失败: " + err.Error())
		return nil, err
	}
	asnLookupState.reader = reader
	sendLog("ASN 数据使用内置离线快照（构建于 " + asnBuildDate(reader) + "）")
	startASNRefreshLocked()
	return reader, nil
}

func openASNCache() (*maxminddb.Reader, error) {
	reader, err := maxminddb.Open(asnDBFile)
	if err != nil {
		return nil, err
	}
	if err := validateASNReader(reader); err != nil {
		reader.Close()
		return nil, err
	}
	return reader, nil
}

func openEmbeddedASNReader() (*maxminddb.Reader, error) {
	zr, err := gzip.NewReader(bytes.NewReader(embeddedASNGzip))
	if err != nil {
		return nil, fmt.Errorf("解压内置 ASN 快照失败: %w", err)
	}
	defer zr.Close()
	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("读取内置 ASN 快照失败: %w", err)
	}
	reader, err := maxminddb.FromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("解析内置 ASN 快照失败: %w", err)
	}
	if err := validateASNReader(reader); err != nil {
		reader.Close()
		return nil, err
	}
	return reader, nil
}

func validateASNReader(reader *maxminddb.Reader) error {
	if reader.Metadata.DatabaseType != asnDatabaseType {
		return fmt.Errorf("ASN 数据库类型为 %q，期望 %q", reader.Metadata.DatabaseType, asnDatabaseType)
	}
	if reader.Metadata.BuildEpoch == 0 {
		return fmt.Errorf("ASN 数据库缺少构建时间")
	}
	return nil
}

func asnCacheStale(reader *maxminddb.Reader) bool {
	if reader.Metadata.BuildEpoch == 0 {
		return true
	}
	return time.Since(time.Unix(int64(reader.Metadata.BuildEpoch), 0)) > asnCacheMaxAge
}

func asnBuildDate(reader *maxminddb.Reader) string {
	if reader.Metadata.BuildEpoch == 0 {
		return "未知日期"
	}
	return time.Unix(int64(reader.Metadata.BuildEpoch), 0).UTC().Format("2006-01-02")
}

func startASNRefreshLocked() {
	asnLookupState.refreshGeneration++
	generation := asnLookupState.refreshGeneration
	go refreshASNCache(generation)
}

func refreshASNCache(generation uint64) {
	client := managementHTTPClient(asnFetchTimeout)
	for _, url := range asnDBURLs {
		data, err := getURLBytesWithContextClient(context.Background(), url, client)
		if err != nil {
			recordDebugError("asn_download", fmt.Sprintf("%s: %v", url, err))
			continue
		}
		if err := validateASNDatabase(data); err != nil {
			recordDebugError("asn_download", fmt.Sprintf("%s: %v", url, err))
			continue
		}
		written, err := writeASNCacheIfCurrent(generation, data)
		if !written {
			return
		}
		if err != nil {
			recordDebugError("asn_download", "保存 ASN 数据库失败: "+err.Error())
			sendLog("ASN 数据在线更新保存失败: " + err.Error())
			return
		}
		sendLog("ASN 数据库已更新到本地缓存（下次启动生效）")
		return
	}
	recordDebugError("asn_download", "所有 ASN 镜像均不可用")
}

func validateASNDatabase(data []byte) error {
	reader, err := maxminddb.FromBytes(data)
	if err != nil {
		return err
	}
	defer reader.Close()
	return validateASNReader(reader)
}

func writeASNCacheIfCurrent(generation uint64, data []byte) (bool, error) {
	asnLookupState.Lock()
	defer asnLookupState.Unlock()
	if asnLookupState.refreshGeneration != generation {
		return false, nil
	}
	return true, atomicWriteFile(asnDBFile, data, 0644)
}

func resetASNReader() {
	asnLookupState.Lock()
	defer asnLookupState.Unlock()
	asnLookupState.refreshGeneration++
	if asnLookupState.reader != nil {
		asnLookupState.reader.Close()
	}
	asnLookupState.reader = nil
	asnLookupState.err = nil
	asnLookupState.loaded = false
}
