package main

import (
	"reflect"
	"testing"
)

func ipList(rows []cliResultRow) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row["ip"])
	}
	return out
}

func TestSpeedUploadRowsKeepsQualifiedWhenAny(t *testing.T) {
	rows := []cliResultRow{
		{"ip": "1.1.1.1", "speed": "6.00MB/s"},
		{"ip": "2.2.2.2", "speed": "0.20MB/s"},
		{"ip": "3.3.3.3", "speed": "0.05MB/s"},
	}
	got := speedUploadRows(rows, 5.0, 2)
	if want := []string{"1.1.1.1"}; !reflect.DeepEqual(ipList(got), want) {
		t.Fatalf("存在达标行时应只返回达标行，实际 %v", ipList(got))
	}
}

func TestSpeedUploadRowsZeroQualifiedFallsBackToTopN(t *testing.T) {
	rows := []cliResultRow{
		{"ip": "1.1.1.1", "speed": "0.50MB/s"},
		{"ip": "2.2.2.2", "speed": "4.20MB/s"},
		{"ip": "3.3.3.3", "speed": "0.05MB/s"},
		{"ip": "4.4.4.4", "speed": "未测速"},
		{"ip": "5.5.5.5", "speed": "测速失败: timeout"},
		{"ip": "6.6.6.6"},
	}
	got := speedUploadRows(rows, 5.0, 2)
	if want := []string{"2.2.2.2", "1.1.1.1"}; !reflect.DeepEqual(ipList(got), want) {
		t.Fatalf("总达标 0 应回退为速度降序前 2 条，实际 %v", ipList(got))
	}
	got = speedUploadRows(rows, 5.0, 20)
	if want := []string{"2.2.2.2", "1.1.1.1", "3.3.3.3"}; !reflect.DeepEqual(ipList(got), want) {
		t.Fatalf("有效行不足上限时应全部回退且排除无效行，实际 %v", ipList(got))
	}
}

func TestSpeedUploadRowsNoValidSpeedReturnsEmpty(t *testing.T) {
	rows := []cliResultRow{
		{"ip": "1.1.1.1", "speed": "未测速"},
		{"ip": "2.2.2.2", "speed": "测速失败"},
	}
	if got := speedUploadRows(rows, 5.0, 20); len(got) != 0 {
		t.Fatalf("无可解析速度时应返回空，实际 %v", ipList(got))
	}
}

func TestSpeedUploadRowsLimitZeroDisablesFallback(t *testing.T) {
	rows := []cliResultRow{{"ip": "1.1.1.1", "speed": "0.50MB/s"}}
	if got := speedUploadRows(rows, 5.0, 0); got != nil {
		t.Fatalf("测速上限为 0 时不启用回退，实际 %v", ipList(got))
	}
}

func TestTopRowsByMeasuredSpeedStableOnTies(t *testing.T) {
	rows := []cliResultRow{
		{"ip": "1.1.1.1", "speed": "1.00MB/s"},
		{"ip": "2.2.2.2", "speed": "2.00MB/s"},
		{"ip": "3.3.3.3", "speed": "2.00MB/s"},
	}
	got := topRowsByMeasuredSpeed(rows, 3)
	if want := []string{"2.2.2.2", "3.3.3.3", "1.1.1.1"}; !reflect.DeepEqual(ipList(got), want) {
		t.Fatalf("同速行应保持稳定顺序，实际 %v", ipList(got))
	}
	if topRowsByMeasuredSpeed(rows, 0) != nil {
		t.Fatalf("上限为 0 应返回 nil")
	}
	if got := topRowsByMeasuredSpeed(nil, 5); len(got) != 0 {
		t.Fatalf("空输入应返回空，实际 %v", got)
	}
}
