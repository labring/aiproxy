package middleware

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/notify"
)

func GinRecoveryHandler(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			// Check for a broken connection, as it is not really a
			// condition that warrants a panic stack trace.
			var brokenPipe bool
			if ne, ok := err.(*net.OpError); ok {
				var se *os.SyscallError
				if errors.As(ne, &se) {
					seStr := strings.ToLower(se.Error())
					if strings.Contains(seStr, "broken pipe") ||
						strings.Contains(seStr, "connection reset by peer") {
						brokenPipe = true
					}
				}
			}

			fileLine, stack := stack(3)
			httpRequest, _ := httputil.DumpRequest(c.Request, false)

			headers := strings.Split(string(httpRequest), "\r\n")
			for idx, header := range headers {
				current := strings.Split(header, ":")
				if current[0] == "Authorization" {
					headers[idx] = current[0] + ": *"
				}
			}

			headersToStr := strings.Join(headers, "\r\n")
			switch {
			case brokenPipe:
				notify.ErrorThrottle("ginPanicRecovery:"+fileLine,
					time.Minute*5, "Panic Detected",
					fmt.Sprintf("%s\n%s", err, headersToStr))
			case gin.IsDebugging():
				notify.ErrorThrottle("ginPanicRecovery:"+fileLine,
					time.Minute*5, "Panic Detected",
					fmt.Sprintf("[Recovery] panic recovered:\n%s\n%s\n%s",
						headersToStr, err, stack),
				)
			default:
				notify.ErrorThrottle("ginPanicRecovery:"+fileLine,
					time.Minute*5, "Panic Detected",
					fmt.Sprintf("[Recovery] panic recovered:\n%s\n%s",
						err, stack),
				)
			}

			if brokenPipe {
				// If the connection is dead, we can't write a status to it.
				c.Error(err.(error)) //nolint: errcheck
				c.Abort()
			} else {
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}
	}()

	c.Next()
}

// stack returns a nicely formatted stack frame, skipping skip frames.
func stack(skip int) (fileLine string, stack []byte) {
	buf := new(bytes.Buffer) // the returned data
	// As we loop, we open files and read them. These variables record the currently
	// loaded file.
	var (
		lines    [][]byte
		lastFile string
	)

	for i := skip; ; i++ { // Skip the expected number of frames
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Print this much at least.  If we can't find the source, it won't show.
		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)

		if fileLine == "" {
			fileLine = fmt.Sprintf("%s:%d", file, line)
		}

		if file != lastFile {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}

			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}

		fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
	}

	return fileLine, buf.Bytes()
}

// source returns a space-trimmed slice of the n'th line.
func source(lines [][]byte, n int) []byte {
	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed
	if n < 0 || n >= len(lines) {
		return dunno
	}

	return bytes.TrimSpace(lines[n])
}

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
)

// function returns, if possible, the name of the function containing the PC.
func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}

	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Also the package path might contain dot (e.g. code.google.com/...),
	// so first eliminate the path prefix
	if lastSlash := bytes.LastIndex(name, slash); lastSlash >= 0 {
		name = name[lastSlash+1:]
	}

	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}

	name = bytes.ReplaceAll(name, centerDot, dot)

	return name
}
