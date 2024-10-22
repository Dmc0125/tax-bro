package logger

import (
	"fmt"
	"log"
	"tax-bro/pkg/utils"
)

func printStackTrace(st string) {
	fmt.Printf("\nStacktrace:\n%s", st)
}

func Log(v ...any) {
	st := utils.GetStackTrace()
	log.Print(v...)
	printStackTrace(st)
}

func Logf(format string, v ...any) {
	st := utils.GetStackTrace()
	log.Printf(format, v...)
	printStackTrace(st)
}

func Logln(v ...any) {
	st := utils.GetStackTrace()
	log.Println(v...)
	printStackTrace(st)
}
