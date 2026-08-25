package model

import (
	"errors"
	"fmt"
)

// 领域错误：所有错误在此统一定义，HTTP 层据此映射状态码，避免散落魔法字符串。
var (
	ErrNotFound          = errors.New("resource not found")
	ErrConflict          = errors.New("resource conflict")
	ErrInvalidInput      = errors.New("invalid input")
	ErrSealedTrial       = errors.New("trial is sealed, input modification forbidden")
	ErrTrialNotReady     = errors.New("trial status does not permit this operation")
	ErrMixedUnits        = errors.New("temperature unit mixed within one trial")
	ErrCurveDuplicate    = errors.New("duplicate curve content (same hash)")
	ErrCurveUnsorted     = errors.New("curve temperatures must be strictly increasing")
	ErrSamplingTooCoarse = errors.New("sampling interval exceeds allowed maximum")
	ErrEmptyCurve        = errors.New("curve must contain at least two points")
	ErrOverlapUnresolved = errors.New("overlapping peak must be resolved before confirmation")
	ErrNoActivePrior     = errors.New("no active polymorph prior matches this peak")
	ErrSnapshotFrozen    = errors.New("snapshot is published and immutable")
	ErrStateTransition   = errors.New("illegal state transition")
)

// DomainError 包装领域错误与附加上下文，便于 API 层输出可读消息。
type DomainError struct {
	Kind error
	Msg  string
}

func (e *DomainError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return e.Kind.Error()
}

func (e *DomainError) Unwrap() error { return e.Kind }

// E 快速构造带上下文的领域错误。
func E(kind error, format string, args ...any) error {
	return &DomainError{Kind: kind, Msg: fmt.Sprintf(format, args...)}
}

// IsKind 判断错误是否属于某个领域错误类别。
func IsKind(err error, kind error) bool {
	return errors.Is(err, kind)
}

// TrialStatusOrder 定义试验状态的合法推进顺序（用于状态机校验）。
var TrialStatusOrder = []string{
	TrialReceiving,
	TrialPending,
	TrialNeedsReview,
	TrialConfirmed,
	TrialSealed,
}

// CanTransition 判断试验状态 s -> t 是否合法（按顺序只进不退，可跳级，封存终态）。
func CanTransition(s, t string) bool {
	if s == t {
		return true
	}
	if s == TrialSealed {
		return false // 封存终态
	}
	si, ti := -1, -1
	for i, st := range TrialStatusOrder {
		if st == s {
			si = i
		}
		if st == t {
			ti = i
		}
	}
	return si >= 0 && ti > si
}

// ValidTrialStatus 校验状态值合法。
func ValidTrialStatus(s string) bool {
	for _, st := range TrialStatusOrder {
		if st == s {
			return true
		}
	}
	return false
}
