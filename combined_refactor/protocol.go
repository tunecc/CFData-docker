package main

import "encoding/json"

type wsRequest struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type startTaskRequest struct {
	IPType             int     `json:"ipType"`
	Threads            int     `json:"threads"`
	Port               int     `json:"port"`
	Delay              int     `json:"delay"`
	ScanMode           string  `json:"scanMode"`
	AutoSpeed          bool    `json:"autoSpeed"`
	OfficialTargetDC   string  `json:"officialTargetDC"`
	OfficialDCFromLib  bool    `json:"officialDcFromLibrary"`
	OfficialStopCount  *int    `json:"officialStopCount"`
	OfficialSpeedPort  int     `json:"officialSpeedPort"`
	OfficialSpeedURL   string  `json:"officialSpeedURL"`
	OfficialSpeedMin   float64 `json:"officialSpeedMin"`
	OfficialSpeedLimit int     `json:"officialSpeedLimit"`
	R2AutoUpload       bool    `json:"r2AutoUpload"`
}

type startDCLibraryScanRequest struct {
	IPType   int    `json:"ipType"`
	Threads  int    `json:"threads"`
	Port     int    `json:"port"`
	Delay    int    `json:"delay"`
	ScanMode string `json:"scanMode"`
}

type startTestRequest struct {
	DC            string `json:"dc"`
	Port          int    `json:"port"`
	Delay         int    `json:"delay"`
	ScanMode      string `json:"scanMode"`
	DeferR2Upload bool   `json:"deferR2Upload"`
	R2AutoUpload  bool   `json:"r2AutoUpload"`
}

type startSpeedTestRequest struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
	URL  string `json:"url"`
}

type startOfficialSpeedBatchRequest struct {
	Port         int          `json:"port"`
	URL          string       `json:"url"`
	SpeedMin     float64      `json:"speedMin"`
	SpeedLimit   int          `json:"speedLimit"`
	Results      []TestResult `json:"results"`
	SkipTested   bool         `json:"skipTested"`
	R2AutoUpload bool         `json:"r2AutoUpload"`
}

type startNSBTaskRequest struct {
	FileName     string  `json:"fileName"`
	FileContent  string  `json:"fileContent"`
	SourceURL    string  `json:"sourceURL"`
	OutFile      string  `json:"outFile"`
	MaxThreads   int     `json:"maxThreads"`
	FallbackPort int     `json:"fallbackPort"`
	SpeedTest    int     `json:"speedTest"`
	SpeedURL     string  `json:"speedURL"`
	EnableTLS    bool    `json:"enableTLS"`
	Delay        int     `json:"delay"`
	ResultLimit  int     `json:"resultLimit"`
	DC           string  `json:"dc"`
	SpeedMin     float64 `json:"speedMin"`
	SpeedLimit   int     `json:"speedLimit"`
	Compact      bool    `json:"compact"`
	ScanMode     string  `json:"scanMode"`
	RelinkHost   string  `json:"relinkHost"`
	R2AutoUpload bool    `json:"r2AutoUpload"`
}

type startNSBReallinkRequest struct {
	Results      []nsbScanMessage `json:"results"`
	RelinkHost   string           `json:"relinkHost"`
	OutFile      string           `json:"outFile"`
	SpeedTest    int              `json:"speedTest"`
	SpeedURL     string           `json:"speedURL"`
	EnableTLS    bool             `json:"enableTLS"`
	SpeedMin     float64          `json:"speedMin"`
	SpeedLimit   int              `json:"speedLimit"`
	Compact      bool             `json:"compact"`
	ScanMode     string           `json:"scanMode"`
	R2AutoUpload bool             `json:"r2AutoUpload"`
}

type startNSBSpeedBatchRequest struct {
	Results      []nsbScanMessage `json:"results"`
	SpeedTest    int              `json:"speedTest"`
	SpeedURL     string           `json:"speedURL"`
	EnableTLS    bool             `json:"enableTLS"`
	SpeedMin     float64          `json:"speedMin"`
	SpeedLimit   int              `json:"speedLimit"`
	SkipTested   bool             `json:"skipTested"`
	Compact      bool             `json:"compact"`
	R2AutoUpload bool             `json:"r2AutoUpload"`
}

type edgetunnelUploadRequest struct {
	Host     string `json:"host"`
	Password string `json:"password"`
	Content  string `json:"content"`
	Mode     string `json:"mode"`
	Silent   bool   `json:"silent"`
}

type githubUploadRequest struct {
	Token   string `json:"token"`
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Branch  string `json:"branch"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Content string `json:"content"`
	Silent  bool   `json:"silent"`
}

// taskConfigUpdateRequest 保存任务参数的请求。指针为 nil 表示不修改该字段。
type taskConfigUpdateRequest struct {
	Mode               *string  `json:"mode,omitempty"`
	OfficialScanMode   *string  `json:"officialScanMode,omitempty"`
	OfficialIPType     *int     `json:"officialIpType,omitempty"`
	OfficialPort       *int     `json:"officialPort,omitempty"`
	OfficialThreads    *int     `json:"officialThreads,omitempty"`
	OfficialDelay      *int     `json:"officialDelay,omitempty"`
	OfficialSpeedMin   *float64 `json:"officialSpeedMin,omitempty"`
	OfficialSpeedLimit *int     `json:"officialSpeedLimit,omitempty"`
	OfficialAutoSpeed  *bool    `json:"officialAutoSpeed,omitempty"`
	OfficialTargetDC   *string  `json:"officialTargetDC,omitempty"`
	OfficialStopCount  *int     `json:"officialStopCount,omitempty"`
	OfficialSpeedURL   *string  `json:"officialSpeedUrl,omitempty"`
	NSBScanMode        *string  `json:"nsbScanMode,omitempty"`
	NSBThreads         *int     `json:"nsbThreads,omitempty"`
	NSBDelay           *int     `json:"nsbDelay,omitempty"`
	NSBResultLimit     *int     `json:"nsbResultLimit,omitempty"`
	NSBFallbackPort    *int     `json:"nsbFallbackPort,omitempty"`
	NSBSpeedTest       *int     `json:"nsbSpeedTest,omitempty"`
	NSBSpeedMin        *float64 `json:"nsbSpeedMin,omitempty"`
	NSBSpeedLimit      *int     `json:"nsbSpeedLimit,omitempty"`
	NSBEnableSpeed     *bool    `json:"nsbEnableSpeed,omitempty"`
	NSBTargetDC        *string  `json:"nsbTargetDC,omitempty"`
	NSBTLS             *bool    `json:"nsbTls,omitempty"`
	NSBCompact         *bool    `json:"nsbCompact,omitempty"`
	NSBOutFile         *string  `json:"nsbOutFile,omitempty"`
	NSBSpeedURL        *string  `json:"nsbSpeedUrl,omitempty"`
	NSBSourceURL       *string  `json:"nsbSourceUrl,omitempty"`
	NSBRelink          *bool    `json:"nsbRelink,omitempty"`
	NSBRelinkHost      *string  `json:"nsbRelinkHost,omitempty"`
}

func (r *taskConfigUpdateRequest) toTaskConfigUpdate() *taskConfigUpdate {
	return &taskConfigUpdate{
		Mode:               r.Mode,
		OfficialScanMode:   r.OfficialScanMode,
		OfficialIPType:     r.OfficialIPType,
		OfficialPort:       r.OfficialPort,
		OfficialThreads:    r.OfficialThreads,
		OfficialDelay:      r.OfficialDelay,
		OfficialSpeedMin:   r.OfficialSpeedMin,
		OfficialSpeedLimit: r.OfficialSpeedLimit,
		OfficialAutoSpeed:  r.OfficialAutoSpeed,
		OfficialTargetDC:   r.OfficialTargetDC,
		OfficialStopCount:  r.OfficialStopCount,
		OfficialSpeedURL:   r.OfficialSpeedURL,
		NSBScanMode:        r.NSBScanMode,
		NSBThreads:         r.NSBThreads,
		NSBDelay:           r.NSBDelay,
		NSBResultLimit:     r.NSBResultLimit,
		NSBFallbackPort:    r.NSBFallbackPort,
		NSBSpeedTest:       r.NSBSpeedTest,
		NSBSpeedMin:        r.NSBSpeedMin,
		NSBSpeedLimit:      r.NSBSpeedLimit,
		NSBEnableSpeed:     r.NSBEnableSpeed,
		NSBTargetDC:        r.NSBTargetDC,
		NSBTLS:             r.NSBTLS,
		NSBCompact:         r.NSBCompact,
		NSBOutFile:         r.NSBOutFile,
		NSBSpeedURL:        r.NSBSpeedURL,
		NSBSourceURL:       r.NSBSourceURL,
		NSBRelink:          r.NSBRelink,
		NSBRelinkHost:      r.NSBRelinkHost,
	}
}
