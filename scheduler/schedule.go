package scheduler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// parseDurationToSeconds parses an ISO 8601 duration string (e.g., "PT30S", "PT2M", "PT1H30M10S")
// and returns the total duration in seconds.
func parseDurationToSeconds(duration string) (int, error) {
	re := regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)
	matches := re.FindStringSubmatch(strings.ToUpper(duration))
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

// ParseScheduleScript parses a multi-line schedule script into ScheduleEntry values.
// Each line has the format: PT<duration>|<eventName(description)>|<key=value;key=value>
// Empty lines and lines starting with # are skipped.
func ParseScheduleScript(script string) ([]ScheduleEntry, error) {
	var entries []ScheduleEntry

	lines := strings.Split(script, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid schedule line (need at least duration|event): %s", line)
		}

		// Parse duration
		delaySec, err := parseDurationToSeconds(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("schedule line %q: %w", line, err)
		}

		// Parse event name and optional description: "eventName(description)" or just "eventName"
		eventPart := strings.TrimSpace(parts[1])
		eventName, description := parseEventNameDescription(eventPart)

		// Parse optional settings
		settings := make(map[string]string)
		if len(parts) == 3 && strings.TrimSpace(parts[2]) != "" {
			for _, kv := range strings.Split(parts[2], ";") {
				kv = strings.TrimSpace(kv)
				if kv == "" {
					continue
				}
				eqIdx := strings.IndexByte(kv, '=')
				if eqIdx < 0 {
					return nil, fmt.Errorf("invalid setting %q in schedule line: %s", kv, line)
				}
				settings[strings.TrimSpace(kv[:eqIdx])] = strings.TrimSpace(kv[eqIdx+1:])
			}
		}

		entries = append(entries, ScheduleEntry{
			Delay:       delaySec,
			EventName:   eventName,
			Description: description,
			Settings:    settings,
		})
	}

	return entries, nil
}

// parseEventNameDescription splits "eventName(description)" into name and description.
func parseEventNameDescription(s string) (string, string) {
	idx := strings.IndexByte(s, '(')
	if idx < 0 {
		return s, ""
	}
	name := s[:idx]
	desc := strings.TrimSuffix(s[idx+1:], ")")
	return name, desc
}
