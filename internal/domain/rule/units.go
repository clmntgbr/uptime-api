package rule

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseDurationMS reads "800 ms", "800", "2 s" or "1.5s" as milliseconds.
func ParseDurationMS(value string) (int64, error) {
	m := durationValue.FindStringSubmatch(strings.ToLower(strings.TrimSpace(value)))
	if m == nil {
		return 0, fmt.Errorf("%w: %q is not a duration (800 ms, 2 s)", ErrInvalid, value)
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	if m[2] == "s" {
		n *= 1000
	}
	if n <= 0 {
		return 0, fmt.Errorf("%w: duration must be positive", ErrInvalid)
	}
	return int64(math.Round(n)), nil
}

// ParseSizeBytes reads "512", "100 B", "64 KB" or "1.5 MB" as bytes.
func ParseSizeBytes(value string) (int64, error) {
	m := sizeValue.FindStringSubmatch(strings.ToLower(strings.TrimSpace(value)))
	if m == nil {
		return 0, fmt.Errorf("%w: %q is not a size (100 B, 64 KB, 1 MB)", ErrInvalid, value)
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	switch m[2] {
	case "kb", "ko":
		n *= 1024
	case "mb", "mo":
		n *= 1024 * 1024
	}
	if n <= 0 {
		return 0, fmt.Errorf("%w: size must be positive", ErrInvalid)
	}
	return int64(math.Round(n)), nil
}

// FormatBytes renders a byte count with the largest exact unit.
func FormatBytes(n int64) string {
	switch {
	case n >= 1024*1024 && n%(1024*1024) == 0:
		return fmt.Sprintf("%d MB", n/(1024*1024))
	case n >= 1024 && n%1024 == 0:
		return fmt.Sprintf("%d KB", n/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
