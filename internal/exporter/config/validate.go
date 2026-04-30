package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type cronField struct {
	any   bool
	step  int
	value int
}

type CronExpr struct {
	minute cronField
	hour   cronField
	dom    cronField
	month  cronField
	dow    cronField
}

func ParseCron(expr string) (CronExpr, error) {
	parts := strings.Fields(strings.TrimSpace(expr))
	if len(parts) != 5 {
		return CronExpr{}, fmt.Errorf("cron expression %q must contain 5 fields", expr)
	}
	minute, err := parseCronField(parts[0], 0, 59)
	if err != nil {
		return CronExpr{}, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseCronField(parts[1], 0, 23)
	if err != nil {
		return CronExpr{}, fmt.Errorf("hour: %w", err)
	}
	dom, err := parseCronField(parts[2], 1, 31)
	if err != nil {
		return CronExpr{}, fmt.Errorf("day-of-month: %w", err)
	}
	month, err := parseCronField(parts[3], 1, 12)
	if err != nil {
		return CronExpr{}, fmt.Errorf("month: %w", err)
	}
	dow, err := parseCronField(parts[4], 0, 6)
	if err != nil {
		return CronExpr{}, fmt.Errorf("day-of-week: %w", err)
	}
	return CronExpr{
		minute: minute,
		hour:   hour,
		dom:    dom,
		month:  month,
		dow:    dow,
	}, nil
}

func (c CronExpr) Next(after time.Time) time.Time {
	candidate := after.UTC().Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 366*24*60; i++ {
		if c.matches(candidate) {
			return candidate
		}
		candidate = candidate.Add(time.Minute)
	}
	return candidate
}

func (c CronExpr) matches(t time.Time) bool {
	return matchCronField(c.minute, t.Minute()) &&
		matchCronField(c.hour, t.Hour()) &&
		matchCronField(c.dom, t.Day()) &&
		matchCronField(c.month, int(t.Month())) &&
		matchCronField(c.dow, int(t.Weekday()))
}

func parseCronField(raw string, min, max int) (cronField, error) {
	switch {
	case raw == "*":
		return cronField{any: true}, nil
	case strings.HasPrefix(raw, "*/"):
		step, err := strconv.Atoi(strings.TrimPrefix(raw, "*/"))
		if err != nil || step < 1 {
			return cronField{}, fmt.Errorf("invalid step value %q", raw)
		}
		return cronField{step: step}, nil
	default:
		value, err := strconv.Atoi(raw)
		if err != nil {
			return cronField{}, fmt.Errorf("invalid value %q", raw)
		}
		if value < min || value > max {
			return cronField{}, fmt.Errorf("value %d out of range [%d,%d]", value, min, max)
		}
		return cronField{value: value}, nil
	}
}

func matchCronField(field cronField, value int) bool {
	switch {
	case field.any:
		return true
	case field.step > 0:
		return value%field.step == 0
	default:
		return field.value == value
	}
}

func validateLabels(scope string, labels map[string]string) error {
	for key := range labels {
		if _, ok := AllowedLabelKeys[key]; !ok {
			return fmt.Errorf("%s label %q is not allowed", scope, key)
		}
	}
	return nil
}
