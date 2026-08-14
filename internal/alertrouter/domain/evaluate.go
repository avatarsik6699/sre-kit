package domain

import (
	"fmt"
	"strconv"
)

// EvaluateMetricCondition reports whether value satisfies rule's condition/threshold. Only the
// numeric conditions (">", "<", "=") apply to metrics; "status_is" returns an error since it has
// no meaning against a numeric value.
func EvaluateMetricCondition(condition, threshold string, value float64) (bool, error) {
	switch condition {
	case ConditionGreaterThan, ConditionLessThan, ConditionEquals:
		limit, err := strconv.ParseFloat(threshold, 64)
		if err != nil {
			return false, fmt.Errorf("alertrouter: threshold %q is not numeric: %w", threshold, err)
		}
		switch condition {
		case ConditionGreaterThan:
			return value > limit, nil
		case ConditionLessThan:
			return value < limit, nil
		default:
			return value == limit, nil
		}
	case ConditionStatusIs:
		return false, fmt.Errorf("alertrouter: condition %q does not apply to metrics", condition)
	default:
		return false, fmt.Errorf("alertrouter: unknown condition %q", condition)
	}
}

// EvaluateCheckCondition reports whether status satisfies rule's condition/threshold. Only
// "status_is" applies to checks — a check's status ("ok"/"warn"/"critical") isn't numeric, so the
// comparison conditions have no meaning here.
func EvaluateCheckCondition(condition, threshold, status string) (bool, error) {
	switch condition {
	case ConditionStatusIs:
		return status == threshold, nil
	case ConditionGreaterThan, ConditionLessThan, ConditionEquals:
		return false, fmt.Errorf("alertrouter: condition %q does not apply to checks", condition)
	default:
		return false, fmt.Errorf("alertrouter: unknown condition %q", condition)
	}
}
