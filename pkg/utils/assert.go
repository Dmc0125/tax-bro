package utils

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
)

func getStackTrace() string {
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
		stackTrace := getStackTrace()
		e := os.Getenv("RUNTIME_ENV")

		if e == "integration_tests" {
			log.Panicf("Assertion failed: %s\nStack trace: %s\n", msg, stackTrace)
		} else {
			log.Fatalf("Assertion failed: %s\nStack trace: %s\n", msg, stackTrace)
		}
	}
}
