package utils

import (
	"fmt"
	"log"
	"runtime"
	"strings"
)

func GetStackTrace() string {
	pc := make([]uintptr, 10)
	n := runtime.Callers(2, pc)
	if n == 0 {
		return "Unable to retrieve stack trace"
	}

	pc = pc[:n]
	frames := runtime.CallersFrames(pc)

	var stackTrace strings.Builder
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&stackTrace, "%s\n\tat %s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}

	return stackTrace.String()
}

func Assert(cond bool, msg string) {
	if !cond {
		stackTrace := GetStackTrace()
		log.Fatalf("Assertion failed: %s\nStack trace: %s\n", msg, stackTrace)
	}
}
