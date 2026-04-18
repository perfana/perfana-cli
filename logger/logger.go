package logger

import (
	"fmt"
	"log"
	"strings"
)

func Info(msg string, args ...any) {
	log.Println("level=INFO " + msg + formatArgs(args))
}

func Warn(msg string, args ...any) {
	log.Println("level=WARN " + msg + formatArgs(args))
}

func Debug(msg string, args ...any) {
	// Debug is a no-op by default; set log flags to enable if needed.
}

func formatArgs(args []any) string {
	if len(args) == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i+1 < len(args); i += 2 {
		b.WriteString(fmt.Sprintf(" %v=%v", args[i], args[i+1]))
	}
	return b.String()
}
