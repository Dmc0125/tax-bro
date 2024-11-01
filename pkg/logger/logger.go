package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"tax-bro/pkg/utils"
	"time"
)

const timeFormat = "15:04:05.9999Z07:00"
const (
	black        = 30
	red          = 31
	green        = 32
	yellow       = 33
	blue         = 34
	magenta      = 35
	cyan         = 36
	lightGray    = 37
	darkGray     = 90
	lightRed     = 91
	lightGreen   = 92
	lightYellow  = 93
	lightBlue    = 94
	lightMagenta = 95
	lightCyan    = 96
	white        = 97
)

type prettyHandler struct {
	colors      bool
	out         io.Writer
	buf         *bytes.Buffer
	m           *sync.Mutex
	jsonHandler slog.Handler
}

func NewPrettyLogger(path string, level int) {
	out := os.Stdout
	var err error
	if path != "" {
		out, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
		utils.AssertNoErr(err, "unable to create log file")
	}

	jsonBuf := bytes.Buffer{}
	jsonHandler := slog.NewJSONHandler(&jsonBuf, &slog.HandlerOptions{
		Level: slog.Level(level),
	})

	handler := &prettyHandler{
		colors:      path == "",
		out:         out,
		jsonHandler: jsonHandler,
		buf:         &jsonBuf,
		m:           &sync.Mutex{},
	}

	slog.SetDefault(slog.New(handler))
}

func (h *prettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &prettyHandler{}
}

func (h *prettyHandler) WithGroup(group string) slog.Handler {
	return &prettyHandler{}
}

func (h *prettyHandler) colorize(msg string, color int) string {
	if h.colors {
		return fmt.Sprintf("\033[%dm%s\033[0m", color, msg)
	}
	return msg
}

func (h *prettyHandler) colorizeLevel(level string) string {
	color := white
	switch level {
	case "INFO":
		color = cyan
	case "ERROR":
		color = red
	case "WARN":
		color = lightYellow
	case "FATAL":
		color = magenta
	case "DEBUG":
		color = lightRed
	}
	return h.colorize(level, color)
}

func (h *prettyHandler) Handle(ctx context.Context, record slog.Record) error {
	h.m.Lock()
	h.buf.Reset()
	if err := h.jsonHandler.Handle(ctx, record); err != nil {
		return err
	}

	attrs := map[string]interface{}{}
	if err := json.Unmarshal(h.buf.Bytes(), &attrs); err != nil {
		return err
	}

	h.m.Unlock()

	msg := strings.Builder{}

	timeVal := attrs["time"]
	if timeVal != nil {
		timestamp, err := time.Parse(time.RFC3339Nano, timeVal.(string))
		if err != nil {
			return err
		}
		msg.WriteString(timestamp.Format(timeFormat))
		msg.WriteString(" ")
	}
	levelVal := attrs["level"]
	if levelVal != nil {
		msg.WriteString(h.colorizeLevel(levelVal.(string)))
		msg.WriteString(" ")
	}
	msgVal := attrs["msg"]
	if msgVal != nil {
		msg.WriteString(msgVal.(string))
	}

	delete(attrs, "time")
	delete(attrs, "level")
	delete(attrs, "msg")

	if len(attrs) > 0 {
		msg.WriteString(": ")
	}

	sortedAttrs := slices.Sorted(maps.Keys(attrs))

	for _, key := range sortedAttrs {
		msg.WriteString(h.colorize(key, darkGray))
		msg.WriteString(" ")

		val := attrs[key]
		switch val.(type) {
		case string, int, float32, float64, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			msg.WriteString(fmt.Sprintf("%v ", val))
		default:
			msg.WriteString(fmt.Sprintf("%+v ", val))
		}
	}

	msg.WriteString("\n")
	fmt.Printf(msg.String())

	return nil
}

func PrintStackTrace(st string) {
	fmt.Printf("\nStacktrace:\n%s", st)
}

func Log(v ...any) {
	st := utils.GetStackTrace()
	log.Print(v...)
	PrintStackTrace(st)
}

func Logf(format string, v ...any) {
	st := utils.GetStackTrace()
	log.Printf(format, v...)
	PrintStackTrace(st)
}

func Logln(v ...any) {
	st := utils.GetStackTrace()
	log.Println(v...)
	PrintStackTrace(st)
}
