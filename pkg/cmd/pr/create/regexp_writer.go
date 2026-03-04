package create

import (
	"bytes"
	"io"
	"regexp"
)

// NewRegexpWriter creates a RegexpWriter that replaces matches of re with repl in the output.
func NewRegexpWriter(out io.Writer, re *regexp.Regexp, repl string) *RegexpWriter {
	return &RegexpWriter{out: out, re: *re, repl: repl}
}

// RegexpWriter is an io.Writer that filters output by replacing lines matching a regexp.
type RegexpWriter struct {
	out  io.Writer
	re   regexp.Regexp
	repl string
	buf  []byte
}

// Write processes data by applying regexp replacements line by line and writing the result.
func (s *RegexpWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	filtered := []byte{}
	repl := []byte(s.repl)
	lines := bytes.SplitAfter(data, []byte("\n"))

	if len(s.buf) > 0 {
		lines[0] = append(s.buf, lines[0]...)
	}

	for i, line := range lines {
		if i == len(lines) {
			s.buf = line
		} else {
			f := s.re.ReplaceAll(line, repl)
			if len(f) > 0 {
				filtered = append(filtered, f...)
			}
		}
	}

	if len(filtered) != 0 {
		_, err := s.out.Write(filtered)
		if err != nil {
			return 0, err
		}
	}

	return len(data), nil
}

// Flush writes any remaining buffered data after applying regexp replacements.
func (s *RegexpWriter) Flush() (int, error) {
	if len(s.buf) > 0 {
		repl := []byte(s.repl)
		filtered := s.re.ReplaceAll(s.buf, repl)
		if len(filtered) > 0 {
			return s.out.Write(filtered)
		}
	}

	return 0, nil
}
