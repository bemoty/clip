package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

type ProgressReader struct {
	r        io.Reader
	size     int64
	read     int64
	start    time.Time
	lastDraw time.Time
	lastLen  int
	tty      bool
	done     bool
}

func NewProgressReader(r io.Reader, size int64) *ProgressReader {
	return &ProgressReader{
		r:    r,
		size: size,
		tty:  term.IsTerminal(int(os.Stderr.Fd())),
	}
}

func (pr *ProgressReader) Read(buf []byte) (n int, err error) {
	if pr.start.IsZero() {
		pr.start = time.Now()
	}
	n, err = pr.r.Read(buf)
	pr.read += int64(n)
	if pr.tty && time.Since(pr.lastDraw) >= 100*time.Millisecond {
		pr.Render()
		pr.lastDraw = time.Now()
	}
	return n, err
}

func (pr *ProgressReader) Render() {
	const barWidth = 20
	elapsed := time.Since(pr.start)

	var rate float64
	if elapsed >= time.Millisecond {
		rate = float64(pr.read) / elapsed.Seconds()
	}
	rateStr := formatBytes(int64(rate)) + "/s"

	var line string
	if pr.size > 0 {
		pct := float64(pr.read) / float64(pr.size) * 100
		filled := int(pct / 100 * barWidth)
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("#", filled)
		if filled < barWidth {
			bar += strings.Repeat(" ", barWidth-filled-1)
		}
		line = fmt.Sprintf("%s / %s [%s] %.0f%% at %s",
			formatBytes(pr.read), formatBytes(pr.size), bar, pct, rateStr)
	} else {
		line = fmt.Sprintf("%s at %s", formatBytes(pr.read), rateStr)
	}

	_, _ = fmt.Fprintf(os.Stderr, "\r%s", line)
	pr.lastLen = len(line)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func (pr *ProgressReader) Done() {
	if pr.tty && !pr.done {
		_, _ = fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", pr.lastLen))
		pr.done = true
	}
}
