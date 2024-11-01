package utils

import (
	"fmt"
	"os"
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
		fmt.Printf("Assertion failed: %s\nStack trace: %s\n", msg, stackTrace)
		os.Exit(1)
	}
}

func AssertNoErr(err error, msgs ...string) {
	if err != nil {
		stackTrace := GetStackTrace()

		msg := strings.Builder{}
		msg.WriteString("Assertion failed ")
		if len(msgs) > 0 {
			msg.WriteString(msgs[0])
		}
		msg.WriteString(fmt.Sprintf("\nErr: %s\nStack trace: %s\n", err, stackTrace))
		fmt.Print(msg.String())
		os.Exit(1)
	}
}
