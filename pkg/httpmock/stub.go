package httpmock

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Matcher is a function that reports whether an HTTP request matches a stub.
type Matcher func(req *http.Request) bool

// Responder is a function that generates an HTTP response for a matched request.
type Responder func(req *http.Request) (*http.Response, error)

// Stub pairs a Matcher with a Responder and tracks whether the stub has been matched.
type Stub struct {
	Stack     string
	matched   bool
	Matcher   Matcher
	Responder Responder
	exclude   bool
}

// MatchAny is a Matcher that matches every HTTP request.
func MatchAny(*http.Request) bool {
	return true
}

// REST returns a matcher to a request for the HTTP method and URL escaped path p.
// For example, to match a GET request to `/api/v3/repos/octocat/hello-world/`
// use REST("GET", "api/v3/repos/octocat/hello-world")
// To match a GET request to `/user` use REST("GET", "user")
func REST(method, p string) Matcher {
	return func(req *http.Request) bool {
		if !strings.EqualFold(req.Method, method) {
			return false
		}
		return req.URL.EscapedPath() == "/"+p
	}
}

// GraphQL returns a Matcher that matches POST requests to the GraphQL endpoint whose query matches the given regex.
func GraphQL(q string) Matcher {
	re := regexp.MustCompile(q)

	return func(req *http.Request) bool {
		if !strings.EqualFold(req.Method, "POST") {
			return false
		}
		if req.URL.Path != "/graphql" && req.URL.Path != "/api/graphql" {
			return false
		}

		var bodyData struct {
			Query string
		}
		_ = decodeJSONBody(req, &bodyData)

		return re.MatchString(bodyData.Query)
	}
}

// GraphQLMutationMatcher returns a Matcher for GraphQL mutations whose query matches the regex and whose input satisfies cb.
func GraphQLMutationMatcher(q string, cb func(map[string]interface{}) bool) Matcher {
	re := regexp.MustCompile(q)

	return func(req *http.Request) bool {
		if !strings.EqualFold(req.Method, "POST") {
			return false
		}
		if req.URL.Path != "/graphql" && req.URL.Path != "/api/graphql" {
			return false
		}

		var bodyData struct {
			Query     string
			Variables struct {
				Input map[string]interface{}
			}
		}
		_ = decodeJSONBody(req, &bodyData)

		if re.MatchString(bodyData.Query) {
			return cb(bodyData.Variables.Input)
		}

		return false
	}
}

// QueryMatcher returns a Matcher that matches REST requests by method, path, and query parameters.
func QueryMatcher(method string, path string, query url.Values) Matcher {
	return func(req *http.Request) bool {
		if !REST(method, path)(req) {
			return false
		}

		actualQuery := req.URL.Query()

		for param := range query {
			if !(actualQuery.Get(param) == query.Get(param)) {
				return false
			}
		}

		return true
	}
}

func readBody(req *http.Request) ([]byte, error) {
	bodyCopy := &bytes.Buffer{}
	r := io.TeeReader(req.Body, bodyCopy)
	req.Body = io.NopCloser(bodyCopy)
	return io.ReadAll(r)
}

func decodeJSONBody(req *http.Request, dest interface{}) error {
	b, err := readBody(req)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

// StringResponse returns a Responder that replies with a 200 status and the given string body.
func StringResponse(body string) Responder {
	return func(req *http.Request) (*http.Response, error) {
		return httpResponse(200, req, bytes.NewBufferString(body)), nil
	}
}

// BinaryResponse returns a Responder that replies with a 200 status and the given byte slice body.
func BinaryResponse(body []byte) Responder {
	return func(req *http.Request) (*http.Response, error) {
		return httpResponse(200, req, bytes.NewBuffer(body)), nil
	}
}

// WithHost wraps a Matcher to additionally require the request host to match the given host.
func WithHost(matcher Matcher, host string) Matcher {
	return func(req *http.Request) bool {
		if !strings.EqualFold(req.Host, host) {
			return false
		}
		return matcher(req)
	}
}

// WithHeader wraps a Responder to add the specified header to the response.
func WithHeader(responder Responder, header string, value string) Responder {
	return func(req *http.Request) (*http.Response, error) {
		resp, _ := responder(req)
		if resp.Header == nil {
			resp.Header = make(http.Header)
		}
		resp.Header.Set(header, value)
		return resp, nil
	}
}

// StatusStringResponse returns a Responder that replies with the given status code and string body.
func StatusStringResponse(status int, body string) Responder {
	return func(req *http.Request) (*http.Response, error) {
		return httpResponse(status, req, bytes.NewBufferString(body)), nil
	}
}

// JSONResponse returns a Responder that JSON-encodes the given value and replies with a 200 status.
func JSONResponse(body interface{}) Responder {
	return func(req *http.Request) (*http.Response, error) {
		b, _ := json.Marshal(body)
		header := http.Header{
			"Content-Type": []string{"application/json"},
		}
		return httpResponseWithHeader(200, req, bytes.NewBuffer(b), header), nil
	}
}

// StatusJSONResponse turns the given argument into a JSON response.
//
// The argument is not meant to be a JSON string, unless it's intentional.
func StatusJSONResponse(status int, body interface{}) Responder {
	return func(req *http.Request) (*http.Response, error) {
		b, _ := json.Marshal(body)
		header := http.Header{
			"Content-Type": []string{"application/json"},
		}
		return httpResponseWithHeader(status, req, bytes.NewBuffer(b), header), nil
	}
}

// JSONErrorResponse is a type-safe helper to avoid confusion around the
// provided argument.
func JSONErrorResponse(status int, err api.HTTPError) Responder {
	return StatusJSONResponse(status, err)
}

// FileResponse returns a Responder that replies with the contents of the named file.
func FileResponse(filename string) Responder {
	return func(req *http.Request) (*http.Response, error) {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		return httpResponse(200, req, f), nil
	}
}

// RESTPayload returns a Responder that decodes the JSON request body, passes it to cb, and replies with the given status and body.
func RESTPayload(responseStatus int, responseBody string, cb func(payload map[string]interface{})) Responder {
	return func(req *http.Request) (*http.Response, error) {
		bodyData := make(map[string]interface{})
		err := decodeJSONBody(req, &bodyData)
		if err != nil {
			return nil, err
		}
		cb(bodyData)

		header := http.Header{
			"Content-Type": []string{"application/json"},
		}
		return httpResponseWithHeader(responseStatus, req, bytes.NewBufferString(responseBody), header), nil
	}
}

// GraphQLMutation returns a Responder that decodes a GraphQL mutation's input variables, passes them to cb, and replies with body.
func GraphQLMutation(body string, cb func(map[string]interface{})) Responder {
	return func(req *http.Request) (*http.Response, error) {
		var bodyData struct {
			Variables struct {
				Input map[string]interface{}
			}
		}
		err := decodeJSONBody(req, &bodyData)
		if err != nil {
			return nil, err
		}
		cb(bodyData.Variables.Input)

		return httpResponse(200, req, bytes.NewBufferString(body)), nil
	}
}

// GraphQLQuery returns a Responder that decodes a GraphQL query and its variables, passes them to cb, and replies with body.
func GraphQLQuery(body string, cb func(string, map[string]interface{})) Responder {
	return func(req *http.Request) (*http.Response, error) {
		var bodyData struct {
			Query     string
			Variables map[string]interface{}
		}
		err := decodeJSONBody(req, &bodyData)
		if err != nil {
			return nil, err
		}
		cb(bodyData.Query, bodyData.Variables)

		return httpResponse(200, req, bytes.NewBufferString(body)), nil
	}
}

// ScopesResponder returns a response with a 200 status code and the given OAuth scopes.
func ScopesResponder(scopes string) func(*http.Request) (*http.Response, error) {
	//nolint:bodyclose
	return StatusScopesResponder(http.StatusOK, scopes)
}

// StatusScopesResponder returns a response with the given status code and OAuth scopes.
func StatusScopesResponder(status int, scopes string) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Request:    req,
			Header: map[string][]string{
				"X-Oauth-Scopes": {scopes},
			},
			Body: io.NopCloser(bytes.NewBufferString("")),
		}, nil
	}
}

func httpResponse(status int, req *http.Request, body io.Reader) *http.Response {
	return httpResponseWithHeader(status, req, body, http.Header{})
}

func httpResponseWithHeader(status int, req *http.Request, body io.Reader, header http.Header) *http.Response {
	return &http.Response{
		StatusCode: status,
		Request:    req,
		Body:       io.NopCloser(body),
		Header:     header,
	}
}
