package util

import (
	"fmt"
	"regexp"
	"strconv"
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

func main() {
	// Example duration strings
	durations := []string{"PT10m", "PT5m", "PT30m", "invalid"}

	for _, dur := range durations {
		minutes, err := ParseISODuration(dur)
		if err != nil {
			fmt.Printf("Error parsing duration '%s': %v\n", dur, err)
		} else {
			fmt.Printf("Duration '%s' is %d minutes.\n", dur, minutes)
		}
	}
}
