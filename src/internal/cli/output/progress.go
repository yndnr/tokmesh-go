// Package output provides output formatting for tokmesh-cli.
package output

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// ProgressBar displays a progress bar for file transfers.
type ProgressBar struct {
	w       io.Writer
	title   string
	total   int64
	current int64
	width   int
	mu      sync.Mutex
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(w io.Writer, title string) *ProgressBar {
	return &ProgressBar{
		w:     w,
		title: title,
		width: 40,
	}
}

// SetTotal sets the total size.
func (p *ProgressBar) SetTotal(total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.total = total
}

// Update updates the progress.
func (p *ProgressBar) Update(current, total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = current
	p.total = total
	p.render()
}

// Increment adds to current progress.
func (p *ProgressBar) Increment(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current += n
	p.render()
}

// Finish completes the progress bar.
func (p *ProgressBar) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = p.total
	p.render()
	fmt.Fprintln(p.w)
}

func (p *ProgressBar) render() {
	if p.total <= 0 {
		fmt.Fprintf(p.w, "\r%s %s", p.title, formatBytes(p.current))
		return
	}

	percent := float64(p.current) / float64(p.total)
	if percent > 1 {
		percent = 1
	}

	filled := int(float64(p.width) * percent)
	empty := p.width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	percentStr := fmt.Sprintf("%3.0f%%", percent*100)

	fmt.Fprintf(p.w, "\r%s [%s] %s (%s/%s)",
		p.title,
		bar,
		percentStr,
		formatBytes(p.current),
		formatBytes(p.total),
	)
}

// formatBytes formats bytes to human readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
