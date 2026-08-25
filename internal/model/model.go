// Package model 定义药物晶型转变热分析判读服务的核心领域实体、
// 状态机常量与错误类型。所有状态流转不变量在此固化，跨包共享。
package model

import "time"

// 温度单位。服务内统一以摄氏度为内部表示，输入允许摄氏/开尔文，
// 但同一试验内禁止混用（见 units.go 校验）。
const (
	UnitCelsius    = "C"
	UnitKelvin     = "K"
	UnitMilliWatt  = "mW"  // DSC 热流单位（差示扫描量热）
	UnitMilliGram  = "mg"  // TGA 质量单位（热重分析）
	UnitPercent    = "%"   // TGA 相对质量
	UnitKelvinPerMin = "K/min" // 升温速率
)

// 试验状态机：receiving -> pending_review -> needs_review -> confirmed -> sealed
const (
	TrialReceiving    = "receiving"     // 接收中：曲线/程序导入阶段
	TrialPending      = "pending_review" // 待判读：基线/峰/事件已生成
	TrialNeedsReview  = "needs_review"  // 需复核：存在重叠不确定或否决事件
	TrialConfirmed    = "confirmed"     // 已确认：判读结论定稿
	TrialSealed       = "sealed"        // 封存：输入冻结，禁止修改
)

// 热分析段状态机：raw -> baseline_corrected / anomalous / duplicate
const (
	SegmentRaw              = "raw"
	SegmentBaselineCorrected = "baseline_corrected"
	SegmentAnomalous        = "anomalous"
	SegmentDuplicate        = "duplicate"
)

// 转变事件状态机：candidate -> overlapping / confirmed / vetoed
const (
	EventCandidate   = "candidate"
	EventOverlapping = "overlapping"
	EventConfirmed   = "confirmed"
	EventVetoed      = "vetoed"
)

// 判读快照状态机：draft -> published -> superseded
const (
	SnapshotDraft      = "draft"
	SnapshotPublished  = "published"
	SnapshotSuperseded = "superseded"
)

// 曲线类型
const (
	CurveDSC = "dsc" // 差示扫描量热：热流-温度
	CurveTGA = "tga" // 热重分析：质量-温度
)

// 事件类型（晶型先验分类）
const (
	EventPolymorph   = "polymorph"   // 晶型转变（无质量损失）
	EventDesolvation = "desolvation" // 溶剂/水脱除（TGA 质量损失）
	EventFusion      = "fusion"      // 熔融（吸热且无质量损失）
	EventUnknown     = "unknown"     // 无法归类
)

// 热流方向（DSC 峰方向约定：吸热为正、放热为负，mW）
const (
	DirectionEndothermic = "endothermic"
	DirectionExothermic  = "exothermic"
)

// Trial 是一次药物样品的完整热分析试验（一个样品一份样，含 DSC/TGA 曲线、升温程序与判读）。
type Trial struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Material    string    `json:"material"`
	BatchNo     string    `json:"batch_no"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Unit        string    `json:"unit"` // 温度单位（C/K），同试验内一致
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SealedAt    *time.Time `json:"sealed_at,omitempty"`
	CurveHash   string    `json:"curve_hash,omitempty"` // 输入曲线的内容指纹（幂等判重）
}

// Point 是曲线上的一个采样点：温度为横轴，信号为纵轴。
type Point struct {
	Temp  float64 `json:"temp"`
	Value float64 `json:"value"`
}

// Curve 是一条热分析曲线（DSC 热流或 TGA 质量）。
type Curve struct {
	ID              string    `json:"id"`
	TrialID         string    `json:"trial_id"`
	Kind            string    `json:"kind"` // dsc | tga
	Name            string    `json:"name"`
	Unit            string    `json:"unit"`
	SampleInterval  float64   `json:"sample_interval"` // 温度采样间隔（K）
	Points          []Point   `json:"points"`
	Hash            string    `json:"hash"`       // SHA-256 内容指纹
	Status          string    `json:"status"`     // raw | baseline_corrected | anomalous | duplicate
	ImportedAt      time.Time `json:"imported_at"`
	MassChangePct   *float64  `json:"mass_change_pct,omitempty"` // TGA 全程质量变化百分比
}

// Program 是升温程序（温度-时间协议），不同试验可复用同一版本。
type Program struct {
	ID         string    `json:"id"`
	TrialID    string    `json:"trial_id"`
	Name       string    `json:"name"`
	StartTemp  float64   `json:"start_temp"`
	EndTemp    float64   `json:"end_temp"`
	RateKPerMin float64  `json:"rate_k_per_min"`
	Version    int       `json:"version"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// Segment 是一次基线校正/分析处理产生的热分析段。
type Segment struct {
	ID        string    `json:"id"`
	TrialID   string    `json:"trial_id"`
	CurveID   string    `json:"curve_id"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	Baseline  string    `json:"baseline"`  // 基线构造摘要（JSON）
	Params    string    `json:"params"`    // 校正参数摘要（JSON）
	CreatedAt time.Time `json:"created_at"`
}

// Peak 是检测出的峰区间：起止索引、起止温度、峰顶、峰高、面积与分离度。
type Peak struct {
	ID         string  `json:"id"`
	TrialID    string  `json:"trial_id"`
	CurveID    string  `json:"curve_id"`
	StartIdx   int     `json:"start_idx"`
	EndIdx     int     `json:"end_idx"`
	StartTemp  float64 `json:"start_temp"`
	EndTemp    float64 `json:"end_temp"`
	PeakTemp   float64 `json:"peak_temp"`
	PeakValue  float64 `json:"peak_value"`
	Direction  string  `json:"direction"`
	Height     float64 `json:"height"`
	Area       float64 `json:"area"`
	Separation float64 `json:"separation"` // 与前一峰的分谷深度（0~1，越小越重叠）
	Overlap    bool    `json:"overlap"`
	Status     string  `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Event 是峰经过晶型先验判定后生成的转变事件。
type Event struct {
	ID          string    `json:"id"`
	TrialID     string    `json:"trial_id"`
	PeakID      string    `json:"peak_id"`
	Kind        string    `json:"kind"`
	Form        string    `json:"form"`     // 匹配到的晶型（如 FormA->FormB）
	OnsetTemp   float64   `json:"onset_temp"`
	PeakTemp    float64   `json:"peak_temp"`
	Confidence  float64   `json:"confidence"` // 0~1
	Status      string    `json:"status"`
	Evidence    string    `json:"evidence"` // 判定证据摘要（JSON）
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Snapshot 是判读结论的不可变快照：发布后冻结输入，可被新版本替代。
type Snapshot struct {
	ID          string    `json:"id"`
	TrialID     string    `json:"trial_id"`
	Version     int       `json:"version"`
	Status      string    `json:"status"`
	Summary     string    `json:"summary"`
	EventIDs    []string  `json:"event_ids"`
	FrozenInputs string   `json:"frozen_inputs"` // 冻结的输入指纹（JSON，含曲线哈希）
	PublishedAt *time.Time `json:"published_at,omitempty"`
	ReplacedBy  string    `json:"replaced_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// PolymorphPrior 是晶型转变先验知识：已知转变的温度窗口与热效应方向。
type PolymorphPrior struct {
	ID            string    `json:"id"`
	FormFrom      string    `json:"form_from"`
	FormTo        string    `json:"form_to"`
	OnsetLow      float64   `json:"onset_low"`
	OnsetHigh     float64   `json:"onset_high"`
	Direction     string    `json:"direction"`   // endothermic | exothermic
	MaxMassLossPct float64  `json:"max_mass_loss_pct"` // 允许的最大质量损失（晶型转变应≈0）
	Note          string    `json:"note"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

// Evidence 是事件判定的证据摘要，序列化进 Event.Evidence。
type Evidence struct {
	MatchedPrior   string   `json:"matched_prior,omitempty"`
	PeakTemp       float64  `json:"peak_temp"`
	Direction      string   `json:"direction"`
	MassLossPct    float64  `json:"mass_loss_pct,omitempty"`
	InOnsetWindow  bool     `json:"in_onset_window"`
	DirectionMatch bool     `json:"direction_match"`
	MassLossOK     bool     `json:"mass_loss_ok"`
	Reasons        []string `json:"reasons"`
}
