package test

import (
	"bytes"
	"regexp"
)

// TODO copypasta from command package
type CmdOut struct {
	OutBuf     *bytes.Buffer
	ErrBuf     *bytes.Buffer
	BrowsedURL string
}

// String returns the string representation of CmdOut.
func (c CmdOut) String() string {
	return c.OutBuf.String()
}

// Stderr performs the Stderr operation on CmdOut.
func (c CmdOut) Stderr() string {
	return c.ErrBuf.String()
}

// OutputStub implements a simple utils.Runnable
type OutputStub struct {
	Out   []byte
	Error error
}

// Output runs the OutputStub and returns its output.
func (s OutputStub) Output() ([]byte, error) {
	if s.Error != nil {
		return s.Out, s.Error
	}
	return s.Out, nil
}

// Run executes the OutputStub command.
func (s OutputStub) Run() error {
	if s.Error != nil {
		return s.Error
	}
	return nil
}

// T defines the interface for t.
type T interface {
	Helper()
	Errorf(string, ...interface{})
}

// Deprecated: prefer exact matches for command output
func ExpectLines(t T, output string, lines ...string) {
	t.Helper()
	var r *regexp.Regexp
	for _, l := range lines {
		r = regexp.MustCompile(l)
		if !r.MatchString(output) {
			t.Errorf("output did not match regexp /%s/\n> output\n%s\n", r, output)
			return
		}
	}
}
