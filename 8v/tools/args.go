package tools

import (
	"math"
	"strings"
	"time"
)

const (
	dateLayout     = "2006-01-02"
	monthLayout    = "2006-01"
	dateTimeLayout = "2006-01-02 15:04"
)

func requiredStringArg(args map[string]any, key string) (string, error) {
	value, err := optionalStringArg(args, key)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", badArgument("缺少必填参数 %s", key)
	}

	return value, nil
}

func optionalStringArg(args map[string]any, key string) (string, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return "", nil
	}

	value, ok := raw.(string)
	if !ok {
		return "", badArgument("参数 %s 必须是字符串", key)
	}

	return strings.TrimSpace(value), nil
}

func optionalIntArg(args map[string]any, key string) (*int, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return nil, nil
	}

	value, ok := raw.(float64)
	if !ok || value != math.Trunc(value) {
		return nil, badArgument("参数 %s 必须是整数", key)
	}

	result := int(value)

	return &result, nil
}

func requiredUintArg(args map[string]any, key string) (uint64, error) {
	value, err := optionalUintArg(args, key)
	if err != nil {
		return 0, err
	}
	if value == nil {
		return 0, badArgument("缺少必填参数 %s", key)
	}

	return *value, nil
}

func optionalUintArg(args map[string]any, key string) (*uint64, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return nil, nil
	}

	value, ok := raw.(float64)
	if !ok || value != math.Trunc(value) || value < 0 {
		return nil, badArgument("参数 %s 必须是非负整数", key)
	}

	result := uint64(value)

	return &result, nil
}

func requiredFloatArg(args map[string]any, key string) (float64, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return 0, badArgument("缺少必填参数 %s", key)
	}

	value, ok := raw.(float64)
	if !ok {
		return 0, badArgument("参数 %s 必须是数字", key)
	}

	return value, nil
}

func optionalUintListArg(args map[string]any, key string) ([]uint64, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return nil, nil
	}

	items, ok := raw.([]any)
	if !ok {
		return nil, badArgument("参数 %s 必须是整数数组", key)
	}

	out := make([]uint64, 0, len(items))
	for _, item := range items {
		value, ok := item.(float64)
		if !ok || value != math.Trunc(value) || value <= 0 {
			return nil, badArgument("参数 %s 的每一项都必须是正整数", key)
		}
		out = append(out, uint64(value))
	}

	return out, nil
}

// optionalDateArg parses a YYYY-MM-DD argument into household local midnight.
func (r *Registry) optionalDateArg(args map[string]any, key string) (time.Time, error) {
	value, err := optionalStringArg(args, key)
	if err != nil || value == "" {
		return time.Time{}, err
	}

	parsed, parseErr := time.ParseInLocation(dateLayout, value, r.location)
	if parseErr != nil {
		return time.Time{}, badArgument("参数 %s 必须是 YYYY-MM-DD 格式的日期", key)
	}

	return parsed, nil
}

// optionalDateTimeArg parses a "YYYY-MM-DD HH:MM" argument. It also accepts the
// ISO "YYYY-MM-DDTHH:MM" spelling because models produce both.
func (r *Registry) optionalDateTimeArg(args map[string]any, key string) (time.Time, error) {
	value, err := optionalStringArg(args, key)
	if err != nil || value == "" {
		return time.Time{}, err
	}

	normalized := strings.Replace(value, "T", " ", 1)
	if len(normalized) > len(dateTimeLayout) {
		normalized = normalized[:len(dateTimeLayout)]
	}

	parsed, parseErr := time.ParseInLocation(dateTimeLayout, normalized, r.location)
	if parseErr != nil {
		return time.Time{}, badArgument(
			"参数 %s 必须是 YYYY-MM-DD HH:MM 格式的时间", key,
		)
	}

	return parsed, nil
}

// optionalMonthArg validates a YYYY-MM argument and returns it unchanged; the
// services take the month as a string.
func optionalMonthArg(args map[string]any, key string) (string, error) {
	value, err := optionalStringArg(args, key)
	if err != nil || value == "" {
		return "", err
	}

	if _, parseErr := time.Parse(monthLayout, value); parseErr != nil {
		return "", badArgument("参数 %s 必须是 YYYY-MM 格式的月份", key)
	}

	return value, nil
}

func (r *Registry) today() time.Time {
	now := time.Now().In(r.location)

	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, r.location)
}

func yuan(cents uint64) float64 {
	return math.Round(float64(cents)) / 100
}

func centsFromYuan(amount float64) uint64 {
	return uint64(math.Round(amount * 100))
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}
