package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronExpr is a parsed 5-field cron expression (minute hour day-of-month month
// day-of-week). It supports "*", "?", lists ("1,2"), ranges ("1-5"), steps
// ("*/15", "10-30/5"), month and weekday names ("JAN", "MON"), 7 as Sunday and
// the macros @yearly, @annually, @monthly, @weekly, @daily, @midnight and
// @hourly. Day-of-month and day-of-week follow Vixie cron semantics: when both
// are restricted, a day matches if either matches.
type CronExpr struct {
	expr                          string
	minute, hour, dom, month, dow uint64
	domStar, dowStar              bool
}

var cronMacros = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

var monthNames = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var dowNames = map[string]int{"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6}

// ParseCron parses a cron expression.
func ParseCron(expr string) (*CronExpr, error) {
	spec := strings.TrimSpace(expr)
	if m, ok := cronMacros[strings.ToLower(spec)]; ok {
		spec = m
	}
	fields := strings.Fields(spec)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields, got %d", len(fields))
	}
	c := &CronExpr{expr: expr}
	var err error
	if c.minute, _, err = parseCronField(fields[0], 0, 59, nil); err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	if c.hour, _, err = parseCronField(fields[1], 0, 23, nil); err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	if c.dom, c.domStar, err = parseCronField(fields[2], 1, 31, nil); err != nil {
		return nil, fmt.Errorf("day of month: %w", err)
	}
	if c.month, _, err = parseCronField(fields[3], 1, 12, monthNames); err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	if c.dow, c.dowStar, err = parseCronField(fields[4], 0, 7, dowNames); err != nil {
		return nil, fmt.Errorf("day of week: %w", err)
	}
	// 7 is an alias for Sunday.
	if c.dow&(1<<7) != 0 {
		c.dow = (c.dow | 1) &^ (1 << 7)
	}
	return c, nil
}

// String returns the original expression.
func (c *CronExpr) String() string { return c.expr }

func parseCronField(field string, min, max int, names map[string]int) (uint64, bool, error) {
	var bits uint64
	star := field == "*" || field == "?"
	for _, part := range strings.Split(field, ",") {
		if part == "" {
			return 0, false, fmt.Errorf("empty list element in %q", field)
		}
		rangePart, step := part, 1
		if i := strings.Index(part, "/"); i >= 0 {
			rangePart = part[:i]
			s, err := strconv.Atoi(part[i+1:])
			if err != nil || s <= 0 {
				return 0, false, fmt.Errorf("invalid step in %q", part)
			}
			step = s
		}
		var lo, hi int
		switch {
		case rangePart == "*" || rangePart == "?":
			lo, hi = min, max
		case strings.Contains(rangePart, "-"):
			bounds := strings.SplitN(rangePart, "-", 2)
			var err error
			if lo, err = cronValue(bounds[0], names); err != nil {
				return 0, false, err
			}
			if hi, err = cronValue(bounds[1], names); err != nil {
				return 0, false, err
			}
		default:
			v, err := cronValue(rangePart, names)
			if err != nil {
				return 0, false, err
			}
			lo, hi = v, v
			if strings.Contains(part, "/") {
				hi = max
			}
		}
		if lo < min || hi > max || lo > hi {
			return 0, false, fmt.Errorf("value out of range [%d-%d] in %q", min, max, part)
		}
		for v := lo; v <= hi; v += step {
			bits |= 1 << uint(v)
		}
	}
	return bits, star, nil
}

func cronValue(s string, names map[string]int) (int, error) {
	if names != nil {
		if v, ok := names[strings.ToUpper(s)]; ok {
			return v, nil
		}
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", s)
	}
	return v, nil
}

func (c *CronExpr) dayMatches(t time.Time) bool {
	domOK := c.dom&(1<<uint(t.Day())) != 0
	dowOK := c.dow&(1<<uint(t.Weekday())) != 0
	if c.domStar || c.dowStar {
		return domOK && dowOK
	}
	return domOK || dowOK
}

// Next returns the first activation strictly after t, evaluated in t's
// location. It returns the zero time if none exists within five years.
func (c *CronExpr) Next(t time.Time) time.Time {
	loc := t.Location()
	t = t.Truncate(time.Minute).Add(time.Minute)
	limit := t.AddDate(5, 0, 0)
	for t.Before(limit) {
		if c.month&(1<<uint(t.Month())) == 0 {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc)
			continue
		}
		if !c.dayMatches(t) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
			continue
		}
		if c.hour&(1<<uint(t.Hour())) == 0 {
			next := time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, loc)
			if !next.After(t) { // DST fold: step by absolute time
				next = t.Truncate(time.Hour).Add(time.Hour)
			}
			t = next
			continue
		}
		if c.minute&(1<<uint(t.Minute())) == 0 {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	return time.Time{}
}

// ValidateCronExpr reports whether expr is a valid cron expression. An empty
// expression is valid and marks an on-demand schedule.
func ValidateCronExpr(expr string) bool {
	if IsOnDemand(expr) {
		return true
	}
	_, err := ParseCron(expr)
	return err == nil
}

// ValidateTimezone reports whether tz is a loadable IANA timezone.
func ValidateTimezone(tz string) bool {
	if tz == "" {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

// IsOnDemand reports whether a cron expression means "only when triggered".
func IsOnDemand(cron string) bool { return strings.TrimSpace(cron) == "" }

// ComputeNextCronRun returns the next activation of expr (in timezone tz)
// after the given epoch (or now when after is nil), never earlier than now+1s.
func ComputeNextCronRun(expr, tz string, after *int64) (int64, error) {
	c, err := ParseCron(expr)
	if err != nil {
		return 0, err
	}
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return 0, err
	}
	base := time.Now().In(loc)
	if after != nil {
		base = time.Unix(*after, 0).In(loc)
	}
	next := c.Next(base)
	if next.IsZero() {
		return 0, fmt.Errorf("cron expression %q has no upcoming activation", expr)
	}
	return maxInt64(next.Unix(), time.Now().Unix()+1), nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
