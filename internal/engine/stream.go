package engine

import (
	"fmt"
	"io"
	"strings"

	"github.com/DevAnimecx/deltacode/pkg/models"
)

type StreamRenderer interface {
	Render(chunk models.StreamChunk)
	Done()
}

type TerminalRenderer struct {
	writer  io.Writer
	buffer  strings.Builder
	model   string
}

func NewTerminalRenderer(w io.Writer) *TerminalRenderer {
	return &TerminalRenderer{writer: w}
}

func (r *TerminalRenderer) Render(chunk models.StreamChunk) {
	r.buffer.WriteString(chunk.Content)
	if chunk.Content != "" {
		fmt.Fprint(r.writer, chunk.Content)
	}
	if chunk.Model != "" && r.model == "" {
		r.model = chunk.Model
	}
}

func (r *TerminalRenderer) Done() {
	fmt.Fprintln(r.writer)
}

func (r *TerminalRenderer) FullContent() string {
	return r.buffer.String()
}
