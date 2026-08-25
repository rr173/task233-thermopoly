// Package curve 负责热分析曲线的接收与预处理：内容哈希幂等、
// 点列校验、单位一致性检查与基础统计量计算。
package curve

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"task233-thermopoly/internal/model"
)

// Importer 是曲线导入器：负责把外部传入的点列转换为合法的 Curve 实体。
type Importer struct {
	now func() time.Time
}

func NewImporter() *Importer {
	return &Importer{now: time.Now}
}

// ImportInput 是导入曲线的入参。
type ImportInput struct {
	TrialID string       `json:"trial_id"`
	Kind    string       `json:"kind"`
	Name    string       `json:"name"`
	Unit    string       `json:"unit"`
	Points  []model.Point `json:"points"`
}

// Hash 计算点列内容的 SHA-256 指纹（按温度升序规范化后计算），
// 供幂等判重与快照冻结使用。哈希与导入顺序无关：先排序再序列化。
func Hash(points []model.Point) string {
	cp := make([]model.Point, len(points))
	copy(cp, points)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Temp < cp[j].Temp })
	raw, _ := json.Marshal(cp)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Import 执行导入校验并构造 Curve：
// 1. 曲线类型合法；2. 单位合法且与试验一致；3. 采样间隔合法；
// 4. 点列温度严格递增；5. 计算内容哈希。
func (im *Importer) Import(trialUnit string, in ImportInput) (*model.Curve, error) {
	if in.Kind != model.CurveDSC && in.Kind != model.CurveTGA {
		return nil, model.E(model.ErrInvalidInput, "unsupported curve kind %q", in.Kind)
	}
	if !model.ValidateTemperatureUnit(in.Unit) {
		return nil, model.E(model.ErrInvalidInput, "unsupported temperature unit %q", in.Unit)
	}
	if err := model.CheckUnitConsistency(trialUnit, in.Unit); err != nil {
		return nil, err
	}
	if err := model.ValidateCurvePoints(in.Points); err != nil {
		return nil, err
	}
	interval := SamplingInterval(in.Points)
	if err := model.ValidateSamplingInterval(interval); err != nil {
		return nil, err
	}
	pts := Normalize(in.Points)
	return &model.Curve{
		ID:             NewID("crv"),
		TrialID:        in.TrialID,
		Kind:           in.Kind,
		Name:           in.Name,
		Unit:           in.Unit,
		SampleInterval: interval,
		Points:         pts,
		Hash:           Hash(pts),
		Status:         model.SegmentRaw,
		ImportedAt:     im.now(),
	}, nil
}

// SamplingInterval 计算实际采样间隔：取相邻温度差的中位数，
// 对个别缺失点鲁棒（中位数而非均值）。
func SamplingInterval(points []model.Point) float64 {
	if len(points) < 2 {
		return 0
	}
	deltas := make([]float64, 0, len(points)-1)
	for i := 1; i < len(points); i++ {
		deltas = append(deltas, points[i].Temp-points[i-1].Temp)
	}
	sort.Float64s(deltas)
	return deltas[len(deltas)/2]
}

// Normalize 规范化点列：按温度升序排序并去除重复温度点。
func Normalize(points []model.Point) []model.Point {
	cp := make([]model.Point, len(points))
	copy(cp, points)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Temp < cp[j].Temp })
	out := cp[:0]
	for i, p := range cp {
		if i > 0 && p.Temp == cp[i-1].Temp {
			continue
		}
		out = append(out, p)
	}
	return out
}

// MassChangePct 计算 TGA 曲线全程相对质量变化（百分比），
// 用于晶型转变（≈0）与溶剂脱除（显著）的判别证据。
func MassChangePct(points []model.Point) float64 {
	if len(points) < 2 {
		return 0
	}
	start := points[0].Value
	end := points[len(points)-1].Value
	if math.Abs(start) < 1e-12 {
		return 0
	}
	return (start - end) / math.Abs(start) * 100
}

// Summary 生成曲线统计摘要（供 API 列表与调试使用）。
func Summary(c *model.Curve) map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{
		"id":              c.ID,
		"kind":            c.Kind,
		"name":            c.Name,
		"unit":            c.Unit,
		"points":          len(c.Points),
		"temp_range":      []float64{c.Points[0].Temp, c.Points[len(c.Points)-1].Temp},
		"sample_interval": c.SampleInterval,
		"hash":            c.Hash,
		"status":          c.Status,
	}
}

// NewID 生成唯一 ID（时间戳 + 密码学随机段），跨进程并发安全。
func NewID(prefix string) string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand 失败几乎不可能；退化为纳秒时间戳保证进程内唯一。
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%d-%x", prefix, time.Now().UnixNano(), buf[:])
}
