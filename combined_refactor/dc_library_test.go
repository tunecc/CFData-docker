package main

import (
	"os"
	"testing"
)

func resetDCLibraryStateForTest() {
	dcLibraryState.Lock()
	dcLibraryState.libraries = map[int]*dcLibraryFile{}
	dcLibraryState.Unlock()
}

func TestDCLibrarySaveLoadAndCandidates(t *testing.T) {
	t.Chdir(t.TempDir())
	resetDCLibraryStateForTest()

	oldLocations := locationMap
	locationMap = map[string]location{
		"HKG": {City_zh: "香港", Country: "中国"},
		"NRT": {City_zh: "东京", Country: "日本"},
		"SIN": {City_zh: "新加坡", Country: "新加坡"},
	}
	defer func() { locationMap = oldLocations }()

	if err := saveDCLibrary(4, []ScanResult{
		{IP: "1.1.1.1", DataCenter: "hkg"},
		{IP: "1.0.0.1", DataCenter: "NRT"},
		{IP: "not-an-ip", DataCenter: "HKG"},
	}); err != nil {
		t.Fatalf("saveDCLibrary: %v", err)
	}
	if _, err := os.Stat("dc-library-v4.json"); err != nil {
		t.Fatalf("library file missing: %v", err)
	}
	status := dcLibraryStatusFor(4)
	if !status.Exists || status.IPCount != 2 || status.DCCount != 2 {
		t.Fatalf("unexpected status: %+v", status)
	}
	if _, err := os.Stat("dc-library-v6.json"); !os.IsNotExist(err) {
		t.Fatalf("IPv6 library should not be written, err=%v", err)
	}

	ips := libraryIPsForDC(4, "hkg")
	if len(ips) != 1 || ips[0] != "1.1.1.1" {
		t.Fatalf("libraryIPsForDC = %v, want [1.1.1.1]", ips)
	}
	if ips := libraryIPsForDC(4, "SIN"); len(ips) != 0 {
		t.Fatalf("libraryIPsForDC(SIN) = %v, want empty", ips)
	}

	if err := saveDCLibrary(4, []ScanResult{{IP: "2.2.2.2", DataCenter: "SIN"}}); err != nil {
		t.Fatalf("saveDCLibrary replace: %v", err)
	}
	if ips := libraryIPsForDC(4, "HKG"); len(ips) != 0 {
		t.Fatalf("old snapshot should be replaced, still got %v", ips)
	}
	if status := dcLibraryStatusFor(4); status.IPCount != 1 || status.DCCount != 1 {
		t.Fatalf("replacement status = %+v", status)
	}

	resetDCLibraryStateForTest()
	loadDCLibraries()
	status = dcLibraryStatusFor(4)
	if !status.Exists || status.IPCount != 1 {
		t.Fatalf("library did not survive reload: %+v", status)
	}
}

func TestDCLibraryCorruptFileFallsBackToEmpty(t *testing.T) {
	t.Chdir(t.TempDir())
	resetDCLibraryStateForTest()

	if err := os.WriteFile("dc-library-v4.json", []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	loadDCLibraries()
	status := dcLibraryStatusFor(4)
	if status.Exists || status.IPCount != 0 || len(status.DataCenters) != 0 {
		t.Fatalf("corrupt library should load as empty, got %+v", status)
	}
	if ips := libraryIPsForDC(4, "HKG"); len(ips) != 0 {
		t.Fatalf("corrupt library should yield no candidates, got %v", ips)
	}
}

func TestDCLibraryEmptyStatus(t *testing.T) {
	resetDCLibraryStateForTest()
	status := dcLibraryStatusFor(6)
	if status.Exists || status.IPCount != 0 || status.DCCount != 0 || status.DataCenters == nil {
		t.Fatalf("unexpected empty status: %+v", status)
	}
	payload := dcLibrariesPayload()
	if _, ok := payload["4"]; !ok {
		t.Fatalf("payload missing IPv4 status: %+v", payload)
	}
	if _, ok := payload["6"]; !ok {
		t.Fatalf("payload missing IPv6 status: %+v", payload)
	}
}

func TestResetAllConfigKeepsDCLibrary(t *testing.T) {
	t.Chdir(t.TempDir())
	resetDCLibraryStateForTest()
	if err := saveDCLibrary(4, []ScanResult{{IP: "1.1.1.1", DataCenter: "HKG"}}); err != nil {
		t.Fatal(err)
	}
	session := &appSession{}
	resetAllConfigFiles(session)
	if _, err := os.Stat("dc-library-v4.json"); err != nil {
		t.Fatalf("reset_all_config removed the DC library: %v", err)
	}
}
