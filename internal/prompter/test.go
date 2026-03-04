package prompter

import (
	"fmt"
	"strings"
	"testing"

	ghPrompter "github.com/cli/go-gh/v2/pkg/prompter"
	"github.com/stretchr/testify/assert"
)

// NewMockPrompter creates a MockPrompter for use in tests.
func NewMockPrompter(t *testing.T) *MockPrompter {
	m := &MockPrompter{
		t:                    t,
		PrompterMock:         *ghPrompter.NewMock(t),
		authTokenStubs:       []authTokenStub{},
		confirmDeletionStubs: []confirmDeletionStub{},
		inputHostnameStubs:   []inputHostnameStub{},
		markdownEditorStubs:  []markdownEditorStub{},
	}
	t.Cleanup(m.Verify)
	return m
}

// MockPrompter is a test double for Prompter that uses registered stubs for responses.
type MockPrompter struct {
	t *testing.T
	ghPrompter.PrompterMock
	authTokenStubs             []authTokenStub
	confirmDeletionStubs       []confirmDeletionStub
	inputHostnameStubs         []inputHostnameStub
	markdownEditorStubs        []markdownEditorStub
	multiSelectWithSearchStubs []multiSelectWithSearchStub
}

type authTokenStub struct {
	fn func() (string, error)
}

type confirmDeletionStub struct {
	prompt string
	fn     func(string) error
}

type inputHostnameStub struct {
	fn func() (string, error)
}

type markdownEditorStub struct {
	prompt string
	fn     func(string, string, bool) (string, error)
}

type multiSelectWithSearchStub struct {
	fn func(string, string, []string, []string, func(string) MultiSelectSearchResult) ([]string, error)
}

// AuthToken returns a stubbed authentication token response.
func (m *MockPrompter) AuthToken() (string, error) {
	var s authTokenStub
	if len(m.authTokenStubs) == 0 {
		return "", NoSuchPromptErr("AuthToken")
	}
	s = m.authTokenStubs[0]
	m.authTokenStubs = m.authTokenStubs[1:len(m.authTokenStubs)]
	return s.fn()
}

// ConfirmDeletion returns a stubbed confirm-deletion response.
func (m *MockPrompter) ConfirmDeletion(prompt string) error {
	var s confirmDeletionStub
	if len(m.confirmDeletionStubs) == 0 {
		return NoSuchPromptErr("ConfirmDeletion")
	}
	s = m.confirmDeletionStubs[0]
	m.confirmDeletionStubs = m.confirmDeletionStubs[1:len(m.confirmDeletionStubs)]
	return s.fn(prompt)
}

// InputHostname returns a stubbed hostname input response.
func (m *MockPrompter) InputHostname() (string, error) {
	var s inputHostnameStub
	if len(m.inputHostnameStubs) == 0 {
		return "", NoSuchPromptErr("InputHostname")
	}
	s = m.inputHostnameStubs[0]
	m.inputHostnameStubs = m.inputHostnameStubs[1:len(m.inputHostnameStubs)]
	return s.fn()
}

// MarkdownEditor returns a stubbed markdown editor response.
func (m *MockPrompter) MarkdownEditor(prompt, defaultValue string, blankAllowed bool) (string, error) {
	var s markdownEditorStub
	if len(m.markdownEditorStubs) == 0 {
		return "", NoSuchPromptErr(prompt)
	}
	s = m.markdownEditorStubs[0]
	m.markdownEditorStubs = m.markdownEditorStubs[1:len(m.markdownEditorStubs)]
	if s.prompt != prompt {
		return "", NoSuchPromptErr(prompt)
	}
	return s.fn(prompt, defaultValue, blankAllowed)
}

// MultiSelectWithSearch returns a stubbed multi-select with search response.
func (m *MockPrompter) MultiSelectWithSearch(prompt, searchPrompt string, defaults []string, persistentOptions []string, searchFunc func(string) MultiSelectSearchResult) ([]string, error) {
	var s multiSelectWithSearchStub
	if len(m.multiSelectWithSearchStubs) == 0 {
		return nil, NoSuchPromptErr(prompt)
	}
	s = m.multiSelectWithSearchStubs[0]
	m.multiSelectWithSearchStubs = m.multiSelectWithSearchStubs[1:len(m.multiSelectWithSearchStubs)]
	return s.fn(prompt, searchPrompt, defaults, persistentOptions, searchFunc)
}

// RegisterAuthToken registers a stub function for the AuthToken prompt.
func (m *MockPrompter) RegisterAuthToken(stub func() (string, error)) {
	m.authTokenStubs = append(m.authTokenStubs, authTokenStub{fn: stub})
}

// RegisterConfirmDeletion registers a stub function for the ConfirmDeletion prompt.
func (m *MockPrompter) RegisterConfirmDeletion(prompt string, stub func(string) error) {
	m.confirmDeletionStubs = append(m.confirmDeletionStubs, confirmDeletionStub{prompt: prompt, fn: stub})
}

// RegisterInputHostname registers a stub function for the InputHostname prompt.
func (m *MockPrompter) RegisterInputHostname(stub func() (string, error)) {
	m.inputHostnameStubs = append(m.inputHostnameStubs, inputHostnameStub{fn: stub})
}

// RegisterMarkdownEditor registers a stub function for the MarkdownEditor prompt.
func (m *MockPrompter) RegisterMarkdownEditor(prompt string, stub func(string, string, bool) (string, error)) {
	m.markdownEditorStubs = append(m.markdownEditorStubs, markdownEditorStub{prompt: prompt, fn: stub})
}

// Verify asserts that all registered stubs have been consumed.
func (m *MockPrompter) Verify() {
	errs := []string{}
	if len(m.authTokenStubs) > 0 {
		errs = append(errs, "AuthToken")
	}
	if len(m.confirmDeletionStubs) > 0 {
		errs = append(errs, "ConfirmDeletion")
	}
	if len(m.inputHostnameStubs) > 0 {
		errs = append(errs, "inputHostname")
	}
	if len(m.markdownEditorStubs) > 0 {
		errs = append(errs, "markdownEditorStubs")
	}
	if len(errs) > 0 {
		m.t.Helper()
		m.t.Errorf("%d unmatched calls to %s", len(errs), strings.Join(errs, ","))
	}
}

// AssertOptions asserts that the expected and actual option slices are equal.
func AssertOptions(t *testing.T, expected, actual []string) {
	assert.Equal(t, expected, actual)
}

// IndexFor returns the index of the given answer in the options slice.
func IndexFor(options []string, answer string) (int, error) {
	for ix, a := range options {
		if a == answer {
			return ix, nil
		}
	}
	return -1, NoSuchAnswerErr(answer, options)
}

// IndexesFor returns the indices of the given answers in the options slice.
func IndexesFor(options []string, answers ...string) ([]int, error) {
	indexes := make([]int, len(answers))
	for i, answer := range answers {
		index, err := IndexFor(options, answer)
		if err != nil {
			return nil, err
		}
		indexes[i] = index
	}
	return indexes, nil
}

// NoSuchPromptErr returns an error indicating that no stub was registered for the given prompt.
func NoSuchPromptErr(prompt string) error {
	return fmt.Errorf("no such prompt '%s'", prompt)
}

// NoSuchAnswerErr returns an error indicating that the answer was not found in the options.
func NoSuchAnswerErr(answer string, options []string) error {
	return fmt.Errorf("no such answer '%s' in [%s]", answer, strings.Join(options, ", "))
}
