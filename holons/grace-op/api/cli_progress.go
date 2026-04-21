package api

import (
	"io"

	"github.com/organic-programming/grace-op/internal/progress"
)

func commandProgress(format Format, quiet bool, w io.Writer) *progress.Writer {
	if quiet || format == FormatJSON || w == nil {
		return progress.Silence()
	}
	return progress.New(w)
}

func humanElapsed(p *progress.Writer) string {
	if p == nil {
		return "0s"
	}
	return progress.FormatElapsed(p.Elapsed())
}
