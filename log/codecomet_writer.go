package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
)

const (
	colorBlack = iota + 30
	colorRed
	colorGreen
	colorYellow
	colorBlue
	colorMagenta
	colorCyan
	colorWhite

	colorBold     = 1
	colorDarkGray = 65
)

var consoleBufPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 100))
	},
}

var ModeFieldName = "mode"

var ContextFieldName = "ctx"

var ContextFieldDefault = "core"

const (
	consoleDefaultTimeFormat = time.Kitchen
)

// Formatter transforms the input into a formatted string.
type Formatter func(interface{}) string

// CodecometWriter parses the JSON input and writes it in an
// (optionally) colorized, human-friendly format to Out.
type CodecometWriter struct {
	// Out is the output destination.
	Out io.Writer

	// NoColor disables the colorized output.
	NoColor bool

	// TimeFormat specifies the format for timestamp in output.
	TimeFormat string

	// PartsOrder defines the order of parts in output.
	PartsOrder []string

	// PartsExclude defines parts to not display in output.
	PartsExclude []string

	// FieldsExclude defines contextual fields to not display in output.
	FieldsExclude []string

	FormatTimestamp     Formatter
	FormatLevel         Formatter
	FormatMessage       Formatter
	FormatContext       Formatter
	FormatMode          Formatter
	FormatFieldName     Formatter
	FormatFieldValue    Formatter
	FormatErrFieldName  Formatter
	FormatErrFieldValue Formatter

	FormatExtra func(map[string]interface{}, *bytes.Buffer) error
}

// NewCodecometWriter creates and initializes a new CodecometWriter.
func NewCodecometWriter(options ...func(w *CodecometWriter)) CodecometWriter {
	w := CodecometWriter{
		Out:        os.Stdout,
		TimeFormat: consoleDefaultTimeFormat,
		PartsOrder: consoleDefaultPartsOrder(),
	}

	for _, opt := range options {
		opt(&w)
	}

	// Fix color on Windows
	if w.Out == os.Stdout || w.Out == os.Stderr {
		w.Out = colorable.NewColorable(w.Out.(*os.File))
	}

	return w
}

// Write transforms the JSON input with formatters and appends to w.Out.
func (w CodecometWriter) Write(p []byte) (n int, err error) {
	// Fix color on Windows
	if w.Out == os.Stdout || w.Out == os.Stderr {
		w.Out = colorable.NewColorable(w.Out.(*os.File))
	}

	if w.PartsOrder == nil {
		w.PartsOrder = consoleDefaultPartsOrder()
	}

	buf := consoleBufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		consoleBufPool.Put(buf)
	}()

	var evt map[string]interface{}
	// p = decodeIfBinaryToBytes(p)
	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()
	err = d.Decode(&evt)
	if err != nil {
		return n, fmt.Errorf("cannot decode event: %s", err)
	}

	for _, p := range w.PartsOrder {
		w.writePart(buf, evt, p)
	}

	w.writeFields(evt, buf)

	if w.FormatExtra != nil {
		err = w.FormatExtra(evt, buf)
		if err != nil {
			return n, err
		}
	}

	err = buf.WriteByte('\n')
	if err != nil {
		return n, err
	}

	_, err = buf.WriteTo(w.Out)
	return len(p), err
}

// writeFields appends formatted key-value pairs to buf.
func (w CodecometWriter) writeFields(evt map[string]interface{}, buf *bytes.Buffer) {
	fields := make([]string, 0, len(evt))

	for field := range evt {
		var isExcluded bool

		for _, excluded := range w.FieldsExclude {
			if field == excluded {
				isExcluded = true

				break
			}
		}

		if isExcluded {
			continue
		}

		switch field {
		case zerolog.LevelFieldName, zerolog.TimestampFieldName, zerolog.MessageFieldName, zerolog.CallerFieldName, ContextFieldName, ModeFieldName:
			continue
		}

		fields = append(fields, field)
	}

	sort.Strings(fields)

	// Write space only if something has already been written to the buffer, and if there are fields.
	if buf.Len() > 0 && len(fields) > 0 {
		buf.WriteByte(' ')
		buf.WriteByte(' ')
	}

	// Move the "error" field to the front
	ei := sort.Search(len(fields), func(i int) bool { return fields[i] >= zerolog.ErrorFieldName })
	if ei < len(fields) && fields[ei] == zerolog.ErrorFieldName {
		fields[ei] = ""
		fields = append([]string{zerolog.ErrorFieldName}, fields...)
		xfields := make([]string, 0, len(fields))
		for _, field := range fields {
			if field == "" { // Skip empty fields
				continue
			}
			xfields = append(xfields, field)
		}
		fields = xfields
	}

	for i, field := range fields {
		var fn Formatter
		var fv Formatter

		if field == zerolog.ErrorFieldName {
			if w.FormatErrFieldName == nil {
				fn = consoleDefaultFormatErrFieldName(w.NoColor)
			} else {
				fn = w.FormatErrFieldName
			}

			if w.FormatErrFieldValue == nil {
				fv = consoleDefaultFormatErrFieldValue(w.NoColor)
			} else {
				fv = w.FormatErrFieldValue
			}
		} else {
			if w.FormatFieldName == nil {
				fn = consoleDefaultFormatFieldName(w.NoColor)
			} else {
				fn = w.FormatFieldName
			}

			if w.FormatFieldValue == nil {
				fv = consoleDefaultFormatFieldValue
			} else {
				fv = w.FormatFieldValue
			}
		}

		buf.WriteString(fn(field))

		switch fValue := evt[field].(type) {
		case string:
			if needsQuote(fValue) {
				buf.WriteString(fv(strconv.Quote(fValue)))
			} else {
				buf.WriteString(fv(fValue))
			}
		case json.Number:
			buf.WriteString(fv(fValue))
		default:
			b, err := zerolog.InterfaceMarshalFunc(fValue)
			if err != nil {
				fmt.Fprintf(buf, colorize("[error: %v]", colorRed, w.NoColor), err)
			} else {
				fmt.Fprint(buf, fv(b))
			}
		}

		if i < len(fields)-1 { // Skip space for last field
			buf.WriteByte(' ')
		}
	}
}

// writePart appends a formatted part to buf.
func (w CodecometWriter) writePart(buf *bytes.Buffer, evt map[string]interface{}, p string) {
	var f Formatter

	if w.PartsExclude != nil && len(w.PartsExclude) > 0 {
		for _, exclude := range w.PartsExclude {
			if exclude == p {
				return
			}
		}
	}

	switch p {
	case zerolog.LevelFieldName:
		if w.FormatLevel == nil {
			f = consoleDefaultFormatLevel(w.NoColor)
		} else {
			f = w.FormatLevel
		}
	case zerolog.TimestampFieldName:
		if w.FormatTimestamp == nil {
			f = consoleDefaultFormatTimestamp(w.TimeFormat, w.NoColor)
		} else {
			f = w.FormatTimestamp
		}
	case zerolog.MessageFieldName:
		if w.FormatMessage == nil {
			f = consoleDefaultFormatMessage
		} else {
			f = w.FormatMessage
		}
	case ContextFieldName:
		if w.FormatContext == nil {
			f = consoleDefaultFormatContext
		} else {
			f = w.FormatContext
		}
	case ModeFieldName:
		if w.FormatMode == nil {
			f = consoleDefaultFormatMode
		} else {
			f = w.FormatMode
		}
	// case zerolog.CallerFieldName:
	// 	if w.FormatCaller == nil {
	// 		f = consoleDefaultFormatCaller(w.NoColor)
	// 	} else {
	// 		f = w.FormatCaller
	// 	}
	default:
		if w.FormatFieldValue == nil {
			f = consoleDefaultFormatFieldValue
		} else {
			f = w.FormatFieldValue
		}
	}

	s := f(evt[p])

	if len(s) > 0 {
		if buf.Len() > 0 {
			buf.WriteByte(' ') // Write space only if not the first part
		}
		buf.WriteString(s)
	}
}

// needsQuote returns true when the string s should be quoted in output.
func needsQuote(s string) bool {
	for i := range s {
		if s[i] < 0x20 || s[i] > 0x7e || s[i] == ' ' || s[i] == '\\' || s[i] == '"' {
			return true
		}
	}
	return false
}

// colorize returns the string s wrapped in ANSI code c, unless disabled is true.
func colorize(s interface{}, c int, disabled bool) string {
	if disabled {
		return fmt.Sprintf("%s", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}

// ----- DEFAULT FORMATTERS ---------------------------------------------------

func consoleDefaultPartsOrder() []string {
	return []string{
		zerolog.TimestampFieldName,
		zerolog.LevelFieldName,
		ContextFieldName,
		ModeFieldName,
		zerolog.MessageFieldName,
	}
}

func consoleDefaultFormatTimestamp(timeFormat string, noColor bool) Formatter {
	if timeFormat == "" {
		timeFormat = consoleDefaultTimeFormat
	}
	return func(i interface{}) string {
		t := "<nil>"
		switch tt := i.(type) {
		case string:
			ts, err := time.ParseInLocation(zerolog.TimeFieldFormat, tt, time.Local)
			if err != nil {
				t = tt
			} else {
				t = ts.Local().Format(timeFormat)
			}
		case json.Number:
			i, err := tt.Int64()
			if err != nil {
				t = tt.String()
			} else {
				var sec, nsec int64

				switch zerolog.TimeFieldFormat {
				case zerolog.TimeFormatUnixNano:
					sec, nsec = 0, i
				case zerolog.TimeFormatUnixMicro:
					sec, nsec = 0, int64(time.Duration(i)*time.Microsecond)
				case zerolog.TimeFormatUnixMs:
					sec, nsec = 0, int64(time.Duration(i)*time.Millisecond)
				default:
					sec, nsec = i, 0
				}

				ts := time.Unix(sec, nsec)
				t = ts.Format(timeFormat)
			}
		}
		return colorize(t, colorDarkGray, noColor)
	}
}

func consoleDefaultFormatLevel(noColor bool) Formatter {
	return func(i interface{}) string {
		var l string
		if ll, ok := i.(string); ok {
			switch ll {
			case zerolog.LevelTraceValue:
				l = colorize("TRC", colorMagenta, noColor)
			case zerolog.LevelDebugValue:
				l = colorize("DBG", colorYellow, noColor)
			case zerolog.LevelInfoValue:
				l = colorize("INF", colorGreen, noColor)
			case zerolog.LevelWarnValue:
				l = colorize("WRN", colorRed, noColor)
			case zerolog.LevelErrorValue:
				l = colorize(colorize("ERR", colorRed, noColor), colorBold, noColor)
			case zerolog.LevelFatalValue:
				l = colorize(colorize("FTL", colorRed, noColor), colorBold, noColor)
			case zerolog.LevelPanicValue:
				l = colorize(colorize("PNC", colorRed, noColor), colorBold, noColor)
			default:
				l = colorize(ll, colorBold, noColor)
			}
		} else {
			if i == nil {
				l = colorize("???", colorBold, noColor)
			} else {
				l = strings.ToUpper(fmt.Sprintf("%s", i))[0:3]
			}
		}
		return l
	}
}

func consoleDefaultFormatContext(i interface{}) string {
	if i == nil {
		i = "core"
	}
	return colorize(fmt.Sprintf("%-6s", i), colorBold, false)
}

func consoleDefaultFormatMode(i interface{}) string {
  if i == nil {
    return ""
  }
  return colorize(fmt.Sprintf("%s: ", i), colorRed, false)
}

func consoleDefaultFormatMessage(i interface{}) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%s", i)
}

func consoleDefaultFormatFieldName(noColor bool) Formatter {
	return func(i interface{}) string {
		return colorize(fmt.Sprintf("\n\t\t\t%s=", i), colorCyan, noColor)
	}
}

func consoleDefaultFormatFieldValue(i interface{}) string {
	return fmt.Sprintf("%s", i)
}

func consoleDefaultFormatErrFieldName(noColor bool) Formatter {
	return func(i interface{}) string {
		return colorize(fmt.Sprintf("%s=", i), colorCyan, noColor)
	}
}

func consoleDefaultFormatErrFieldValue(noColor bool) Formatter {
	return func(i interface{}) string {
		return colorize(fmt.Sprintf("%s", i), colorRed, noColor)
	}
}
