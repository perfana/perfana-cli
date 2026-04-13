package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseISODuration parses an ISO 8601 duration string (e.g., "PT10m") and returns the duration in minutes.
// Note that only minutes is supported currently.
func ParseISODuration(duration string) (int, error) {
	// Define a regex to extract minutes. E.g., for "PT10m", this will capture "10".
	re := regexp.MustCompile(`PT(\d+)m`)

	// Match the duration against the regex
	matches := re.FindStringSubmatch(duration)
	if len(matches) != 2 {
		return 0, fmt.Errorf("invalid ISO 8601 duration format: %s", duration)
	}

	// Convert the matched minutes to an integer
	minutes, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("unable to convert minutes: %v", err)
	}

	return minutes, nil
}

// ParseISODurationToSeconds parses an ISO 8601 duration string and returns
// the total duration in seconds. Supports hours (H), minutes (M), and seconds (S),
// case-insensitive. Examples: "PT30S", "PT2M", "PT1H30M10S", "PT15m".
func ParseISODurationToSeconds(duration string) (int, error) {
	upper := strings.ToUpper(duration)
	re := regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)
	matches := re.FindStringSubmatch(upper)
	if matches == nil {
		return 0, fmt.Errorf("invalid ISO 8601 duration format: %s", duration)
	}

	var total int
	if matches[1] != "" {
		h, _ := strconv.Atoi(matches[1])
		total += h * 3600
	}
	if matches[2] != "" {
		m, _ := strconv.Atoi(matches[2])
		total += m * 60
	}
	if matches[3] != "" {
		s, _ := strconv.Atoi(matches[3])
		total += s
	}

	if total == 0 {
		return 0, fmt.Errorf("duration resolves to zero: %s", duration)
	}

	return total, nil
}

// ParseISODurationToTimeDuration parses an ISO 8601 duration string and returns
// a time.Duration value.
func ParseISODurationToTimeDuration(duration string) (time.Duration, error) {
	seconds, err := ParseISODurationToSeconds(duration)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds) * time.Second, nil
}
