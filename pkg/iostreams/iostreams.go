package iostreams

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	ghTerm "github.com/cli/go-gh/v2/pkg/term"
	"github.com/cli/safeexec"
	"github.com/google/shlex"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
)

// DefaultWidth is the fallback terminal width when the actual width cannot be determined.
const DefaultWidth = 80

// ErrClosedPagerPipe is the error returned when writing to a pager that has been closed.
type ErrClosedPagerPipe struct {
	error
}

type fileWriter interface {
	io.Writer
	Fd() uintptr
}

type fileReader interface {
	io.ReadCloser
	Fd() uintptr
}

type term interface {
	IsTerminalOutput() bool
	IsColorEnabled() bool
	Is256ColorSupported() bool
	IsTrueColorSupported() bool
	Theme() string
	Size() (int, int, error)
}

// IOStreams provides access to the standard input, output, and error streams along with
// terminal capabilities and UI helpers such as pagers and progress indicators.
type IOStreams struct {
	term term

	In     fileReader
	Out    fileWriter
	ErrOut fileWriter

	terminalTheme string

	progressIndicatorEnabled bool
	progressIndicator        *spinner.Spinner
	progressIndicatorMu      sync.Mutex
	spinnerDisabled          bool

	alternateScreenBufferEnabled bool
	alternateScreenBufferActive  bool
	alternateScreenBufferMu      sync.Mutex

	stdinTTYOverride  bool
	stdinIsTTY        bool
	stdoutTTYOverride bool
	stdoutIsTTY       bool
	stderrTTYOverride bool
	stderrIsTTY       bool

	colorOverride           bool
	colorEnabled            bool
	colorLabels             bool
	accessibleColorsEnabled bool

	pagerCommand string
	pagerProcess *os.Process

	neverPrompt               bool
	accessiblePrompterEnabled bool

	TempFileOverride *os.File
}

// ColorEnabled reports whether color output is enabled.
func (s *IOStreams) ColorEnabled() bool {
	if s.colorOverride {
		return s.colorEnabled
	}
	return s.term.IsColorEnabled()
}

// ColorSupport256 reports whether the terminal supports 256 colors.
func (s *IOStreams) ColorSupport256() bool {
	if s.colorOverride {
		return s.colorEnabled
	}
	return s.term.Is256ColorSupported()
}

// HasTrueColor reports whether the terminal supports 24-bit true color.
func (s *IOStreams) HasTrueColor() bool {
	if s.colorOverride {
		return s.colorEnabled
	}
	return s.term.IsTrueColorSupported()
}

// ColorLabels reports whether labels should be colored using their RGB hex color.
func (s *IOStreams) ColorLabels() bool {
	return s.colorLabels
}

// DetectTerminalTheme is a utility to call before starting the output pager so that the terminal background
// can be reliably detected.
func (s *IOStreams) DetectTerminalTheme() {
	if !s.ColorEnabled() || s.pagerProcess != nil {
		s.terminalTheme = "none"
		return
	}

	style := os.Getenv("GLAMOUR_STYLE")
	if style != "" && style != "auto" {
		// ensure GLAMOUR_STYLE takes precedence over "light" and "dark" themes
		s.terminalTheme = "none"
		return
	}

	s.terminalTheme = s.term.Theme()
}

// TerminalTheme returns "light", "dark", or "none" depending on the background color of the terminal.
func (s *IOStreams) TerminalTheme() string {
	if s.terminalTheme == "" {
		s.DetectTerminalTheme()
	}

	return s.terminalTheme
}

// SetColorEnabled overrides automatic color detection with the given value.
func (s *IOStreams) SetColorEnabled(colorEnabled bool) {
	s.colorOverride = true
	s.colorEnabled = colorEnabled
}

// SetColorLabels sets whether labels should be colored using their RGB hex color.
func (s *IOStreams) SetColorLabels(colorLabels bool) {
	s.colorLabels = colorLabels
}

// SetStdinTTY overrides automatic stdin TTY detection with the given value.
func (s *IOStreams) SetStdinTTY(isTTY bool) {
	s.stdinTTYOverride = true
	s.stdinIsTTY = isTTY
}

// IsStdinTTY reports whether standard input is connected to a terminal.
func (s *IOStreams) IsStdinTTY() bool {
	if s.stdinTTYOverride {
		return s.stdinIsTTY
	}
	if stdin, ok := s.In.(*os.File); ok {
		return isTerminal(stdin)
	}
	return false
}

// SetStdoutTTY overrides automatic stdout TTY detection with the given value.
func (s *IOStreams) SetStdoutTTY(isTTY bool) {
	s.stdoutTTYOverride = true
	s.stdoutIsTTY = isTTY
}

// IsStdoutTTY reports whether standard output is connected to a terminal.
func (s *IOStreams) IsStdoutTTY() bool {
	if s.stdoutTTYOverride {
		return s.stdoutIsTTY
	}
	// support GH_FORCE_TTY
	if s.term.IsTerminalOutput() {
		return true
	}
	stdout, ok := s.Out.(*os.File)
	return ok && isCygwinTerminal(stdout.Fd())
}

// SetStderrTTY overrides automatic stderr TTY detection with the given value.
func (s *IOStreams) SetStderrTTY(isTTY bool) {
	s.stderrTTYOverride = true
	s.stderrIsTTY = isTTY
}

// IsStderrTTY reports whether standard error is connected to a terminal.
func (s *IOStreams) IsStderrTTY() bool {
	if s.stderrTTYOverride {
		return s.stderrIsTTY
	}
	if stderr, ok := s.ErrOut.(*os.File); ok {
		return isTerminal(stderr)
	}
	return false
}

// SetPager sets the pager command used to paginate output.
func (s *IOStreams) SetPager(cmd string) {
	s.pagerCommand = cmd
}

// GetPager returns the configured pager command.
func (s *IOStreams) GetPager() string {
	return s.pagerCommand
}

// StartPager starts the configured pager process and redirects stdout to it.
func (s *IOStreams) StartPager() error {
	if s.pagerCommand == "" || s.pagerCommand == "cat" || !s.IsStdoutTTY() {
		return nil
	}

	pagerArgs, err := shlex.Split(s.pagerCommand)
	if err != nil {
		return err
	}

	pagerEnv := os.Environ()
	for i := len(pagerEnv) - 1; i >= 0; i-- {
		if strings.HasPrefix(pagerEnv[i], "PAGER=") {
			pagerEnv = append(pagerEnv[0:i], pagerEnv[i+1:]...)
		}
	}
	if _, ok := os.LookupEnv("LESS"); !ok {
		pagerEnv = append(pagerEnv, "LESS=FRX")
	}
	if _, ok := os.LookupEnv("LV"); !ok {
		pagerEnv = append(pagerEnv, "LV=-c")
	}

	pagerExe, err := safeexec.LookPath(pagerArgs[0])
	if err != nil {
		return err
	}
	pagerCmd := exec.Command(pagerExe, pagerArgs[1:]...)
	pagerCmd.Env = pagerEnv
	pagerCmd.Stdout = s.Out
	pagerCmd.Stderr = s.ErrOut
	pagedOut, err := pagerCmd.StdinPipe()
	if err != nil {
		return err
	}
	s.Out = &fdWriteCloser{
		fd:          s.Out.Fd(),
		WriteCloser: &pagerWriter{pagedOut},
	}
	err = pagerCmd.Start()
	if err != nil {
		return err
	}
	s.pagerProcess = pagerCmd.Process
	return nil
}

// StopPager stops the running pager process and restores stdout.
func (s *IOStreams) StopPager() {
	if s.pagerProcess == nil {
		return
	}

	// if a pager was started, we're guaranteed to have a WriteCloser
	_ = s.Out.(io.WriteCloser).Close()
	_, _ = s.pagerProcess.Wait()
	s.pagerProcess = nil
}

// CanPrompt reports whether interactive prompting is possible.
func (s *IOStreams) CanPrompt() bool {
	if s.neverPrompt {
		return false
	}

	return s.IsStdinTTY() && s.IsStdoutTTY()
}

// GetNeverPrompt reports whether prompting has been permanently disabled.
func (s *IOStreams) GetNeverPrompt() bool {
	return s.neverPrompt
}

// SetNeverPrompt permanently enables or disables interactive prompting.
func (s *IOStreams) SetNeverPrompt(v bool) {
	s.neverPrompt = v
}

// GetSpinnerDisabled reports whether the animated spinner is disabled.
func (s *IOStreams) GetSpinnerDisabled() bool {
	return s.spinnerDisabled
}

// SetSpinnerDisabled enables or disables the animated spinner.
func (s *IOStreams) SetSpinnerDisabled(v bool) {
	s.spinnerDisabled = v
}

// StartProgressIndicator starts a progress spinner with no label.
func (s *IOStreams) StartProgressIndicator() {
	s.StartProgressIndicatorWithLabel("")
}

// StartProgressIndicatorWithLabel starts a progress spinner with the given label.
func (s *IOStreams) StartProgressIndicatorWithLabel(label string) {
	if !s.progressIndicatorEnabled {
		return
	}

	if s.spinnerDisabled {
		// If the spinner is disabled, simply print a
		// textual progress indicator and return.
		// This means that s.ProgressIndicator will be nil.
		// See also: the comment on StopProgressIndicator()
		s.startTextualProgressIndicator(label)
		return
	}

	s.progressIndicatorMu.Lock()
	defer s.progressIndicatorMu.Unlock()

	if s.progressIndicator != nil {
		if label == "" {
			s.progressIndicator.Prefix = ""
		} else {
			s.progressIndicator.Prefix = label + " "
		}
		return
	}

	// https://github.com/briandowns/spinner#available-character-sets
	// ⣾ ⣷ ⣽ ⣻ ⡿
	spinnerStyle := spinner.CharSets[11]

	sp := spinner.New(spinnerStyle, 120*time.Millisecond, spinner.WithWriter(s.ErrOut), spinner.WithColor("fgCyan"))
	if label != "" {
		sp.Prefix = label + " "
	}

	sp.Start()
	s.progressIndicator = sp
}

func (s *IOStreams) startTextualProgressIndicator(label string) {
	s.progressIndicatorMu.Lock()
	defer s.progressIndicatorMu.Unlock()

	// Default label when spinner disabled is "Working..."
	if label == "" {
		label = "Working..."
	}

	// Add an ellipsis to the label if it doesn't already have one.
	ellipsis := "..."
	if !strings.HasSuffix(label, ellipsis) {
		label = label + ellipsis
	}

	fmt.Fprintf(s.ErrOut, "%s%s", s.ColorScheme().Cyan(label), "\n")
}

// StopProgressIndicator stops the progress indicator if it is running.
// Note that a textual progess indicator does not create a progress indicator,
// so this method is a no-op in that case.
func (s *IOStreams) StopProgressIndicator() {
	s.progressIndicatorMu.Lock()
	defer s.progressIndicatorMu.Unlock()
	if s.progressIndicator == nil {
		return
	}
	s.progressIndicator.Stop()
	s.progressIndicator = nil
}

// RunWithProgress runs the given function while displaying a labeled progress spinner.
func (s *IOStreams) RunWithProgress(label string, run func() error) error {
	s.StartProgressIndicatorWithLabel(label)
	defer s.StopProgressIndicator()

	return run()
}

// StartAlternateScreenBuffer switches to the terminal's alternate screen buffer.
func (s *IOStreams) StartAlternateScreenBuffer() {
	if s.alternateScreenBufferEnabled {
		s.alternateScreenBufferMu.Lock()
		defer s.alternateScreenBufferMu.Unlock()

		if _, err := fmt.Fprint(s.Out, "\x1b[?1049h"); err == nil {
			s.alternateScreenBufferActive = true

			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt)

			go func() {
				<-ch
				s.StopAlternateScreenBuffer()

				os.Exit(1)
			}()
		}
	}
}

// StopAlternateScreenBuffer switches back from the terminal's alternate screen buffer.
func (s *IOStreams) StopAlternateScreenBuffer() {
	s.alternateScreenBufferMu.Lock()
	defer s.alternateScreenBufferMu.Unlock()

	if s.alternateScreenBufferActive {
		fmt.Fprint(s.Out, "\x1b[?1049l")
		s.alternateScreenBufferActive = false
	}
}

// SetAlternateScreenBufferEnabled enables or disables use of the alternate screen buffer.
func (s *IOStreams) SetAlternateScreenBufferEnabled(enabled bool) {
	s.alternateScreenBufferEnabled = enabled
}

// RefreshScreen clears the terminal screen if stdout is a TTY.
func (s *IOStreams) RefreshScreen() {
	if s.IsStdoutTTY() {
		// Move cursor to 0,0
		fmt.Fprint(s.Out, "\x1b[0;0H")
		// Clear from cursor to bottom of screen
		fmt.Fprint(s.Out, "\x1b[J")
	}
}

// TerminalWidth returns the width of the terminal that controls the process
func (s *IOStreams) TerminalWidth() int {
	w, _, err := s.term.Size()
	if err == nil && w > 0 {
		return w
	}
	return DefaultWidth
}

// ColorScheme returns a ColorScheme configured from the current terminal capabilities.
func (s *IOStreams) ColorScheme() *ColorScheme {
	return &ColorScheme{
		Enabled:       s.ColorEnabled(),
		EightBitColor: s.ColorSupport256(),
		TrueColor:     s.HasTrueColor(),
		Accessible:    s.AccessibleColorsEnabled(),
		ColorLabels:   s.ColorLabels(),
		Theme:         s.TerminalTheme(),
	}
}

// ReadUserFile reads the contents of the given file, or from stdin if the filename is "-".
func (s *IOStreams) ReadUserFile(fn string) ([]byte, error) {
	var r io.ReadCloser
	if fn == "-" {
		r = s.In
	} else {
		var err error
		r, err = os.Open(fn)
		if err != nil {
			return nil, err
		}
	}
	defer r.Close()
	return io.ReadAll(r)
}

// TempFile creates a temporary file, or returns the override if one has been set.
func (s *IOStreams) TempFile(dir, pattern string) (*os.File, error) {
	if s.TempFileOverride != nil {
		return s.TempFileOverride, nil
	}
	return os.CreateTemp(dir, pattern)
}

// SetAccessibleColorsEnabled enables or disables accessible base-16 colors.
func (s *IOStreams) SetAccessibleColorsEnabled(enabled bool) {
	s.accessibleColorsEnabled = enabled
}

// AccessibleColorsEnabled reports whether accessible base-16 colors are enabled.
func (s *IOStreams) AccessibleColorsEnabled() bool {
	return s.accessibleColorsEnabled
}

// SetAccessiblePrompterEnabled enables or disables accessible prompting mode.
func (s *IOStreams) SetAccessiblePrompterEnabled(enabled bool) {
	s.accessiblePrompterEnabled = enabled
}

// AccessiblePrompterEnabled reports whether accessible prompting mode is enabled.
func (s *IOStreams) AccessiblePrompterEnabled() bool {
	return s.accessiblePrompterEnabled
}

// System creates an IOStreams instance connected to the real standard streams.
func System() *IOStreams {
	terminal := ghTerm.FromEnv()

	var stdout fileWriter = os.Stdout
	// On Windows with no virtual terminal processing support, translate ANSI escape
	// sequences to console syscalls.
	if colorableStdout := colorable.NewColorable(os.Stdout); colorableStdout != os.Stdout {
		// Ensure that the file descriptor of the original stdout is preserved.
		stdout = &fdWriter{
			fd:     os.Stdout.Fd(),
			Writer: colorableStdout,
		}
	}

	var stderr fileWriter = os.Stderr
	// On Windows with no virtual terminal processing support, translate ANSI escape
	// sequences to console syscalls.
	if colorableStderr := colorable.NewColorable(os.Stderr); colorableStderr != os.Stderr {
		// Ensure that the file descriptor of the original stderr is preserved.
		stderr = &fdWriter{
			fd:     os.Stderr.Fd(),
			Writer: colorableStderr,
		}
	}

	io := &IOStreams{
		In:           os.Stdin,
		Out:          stdout,
		ErrOut:       stderr,
		pagerCommand: os.Getenv("PAGER"),
		term:         &terminal,
	}

	stdoutIsTTY := io.IsStdoutTTY()
	stderrIsTTY := io.IsStderrTTY()

	if stdoutIsTTY && stderrIsTTY {
		io.progressIndicatorEnabled = true
	}

	if stdoutIsTTY && hasAlternateScreenBuffer(terminal.IsTrueColorSupported()) {
		io.alternateScreenBufferEnabled = true
	}

	return io
}

type fakeTerm struct{}

// IsTerminalOutput reports whether output is a terminal (always false for fakeTerm).
func (t fakeTerm) IsTerminalOutput() bool {
	return false
}

// IsColorEnabled reports whether color is enabled (always false for fakeTerm).
func (t fakeTerm) IsColorEnabled() bool {
	return false
}

// Is256ColorSupported reports whether 256 colors are supported (always false for fakeTerm).
func (t fakeTerm) Is256ColorSupported() bool {
	return false
}

// IsTrueColorSupported reports whether true color is supported (always false for fakeTerm).
func (t fakeTerm) IsTrueColorSupported() bool {
	return false
}

// Theme returns the terminal theme (always empty for fakeTerm).
func (t fakeTerm) Theme() string {
	return ""
}

// Size returns a fixed terminal size of 80 columns for fakeTerm.
func (t fakeTerm) Size() (int, int, error) {
	return 80, -1, nil
}

// Test creates an IOStreams instance backed by in-memory buffers for testing.
func Test() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	io := &IOStreams{
		In: &fdReader{
			fd:         0,
			ReadCloser: io.NopCloser(in),
		},
		Out:    &fdWriter{fd: 1, Writer: out},
		ErrOut: &fdWriter{fd: 2, Writer: errOut},
		term:   &fakeTerm{},
	}
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)
	return io, in, out, errOut
}

func isTerminal(f *os.File) bool {
	return ghTerm.IsTerminal(f) || isCygwinTerminal(f.Fd())
}

func isCygwinTerminal(fd uintptr) bool {
	return isatty.IsCygwinTerminal(fd)
}

// pagerWriter implements a WriteCloser that wraps all EPIPE errors in an ErrClosedPagerPipe type.
type pagerWriter struct {
	io.WriteCloser
}

// Write writes data to the pager, wrapping EPIPE errors in ErrClosedPagerPipe.
func (w *pagerWriter) Write(d []byte) (int, error) {
	n, err := w.WriteCloser.Write(d)
	if err != nil && (errors.Is(err, io.ErrClosedPipe) || isEpipeError(err)) {
		return n, &ErrClosedPagerPipe{err}
	}
	return n, err
}

// fdWriter represents a wrapped stdout Writer that preserves the original file descriptor
type fdWriter struct {
	io.Writer
	fd uintptr
}

// Fd returns the original file descriptor of the wrapped writer.
func (w *fdWriter) Fd() uintptr {
	return w.fd
}

// fdWriteCloser represents a wrapped stdout Writer that preserves the original file descriptor
type fdWriteCloser struct {
	io.WriteCloser
	fd uintptr
}

// Fd returns the original file descriptor of the wrapped write-closer.
func (w *fdWriteCloser) Fd() uintptr {
	return w.fd
}

// fdReader represents a wrapped stdin ReadCloser that preserves the original file descriptor
type fdReader struct {
	io.ReadCloser
	fd uintptr
}

// Fd returns the original file descriptor of the wrapped reader.
func (r *fdReader) Fd() uintptr {
	return r.fd
}
