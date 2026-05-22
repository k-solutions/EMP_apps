package rssreader

import (
	"fmt"
	"strings"
)

// URLError is the interface for any error tied to a specific URL.
type URLError interface {
	error
	URL() string   // the URL that failed
	Unwrap() error // the underlying cause
}

// urlError is the unexported concrete implementation.
type urlError struct {
	url string
	err error
}

func newURLError(url string, err error) URLError {
	return urlError{url: url, err: err}
}

func (e urlError) Error() string { return fmt.Sprintf("%s: %v", e.url, e.err) }
func (e urlError) URL() string   { return e.url }
func (e urlError) Unwrap() error { return e.err }

// ParseError is implemented by errors that aggregate multiple URLErrors.
type ParseError interface {
	error
	Errors() []URLError // individual per-URL failures
	Unwrap() []error    // for errors.As / errors.Is chain walking
}

type parseErrors struct {
	errs []URLError
}

func newParseErrors(errs []URLError) error {
	if len(errs) == 0 {
		return nil
	}
	return &groupedErrors{parseErrors: parseErrors{errs: errs}}
}

func (e *parseErrors) Errors() []URLError { return e.errs }

func (e *parseErrors) Error() string {
	var sb strings.Builder
	sb.WriteString("rssreader parse errors:")
	for _, ue := range e.errs {
		sb.WriteString("\n  - ")
		sb.WriteString(ue.Error())
	}
	return sb.String()
}

func (e *parseErrors) Unwrap() []error {
	errs := make([]error, len(e.errs))
	for i := range e.errs {
		errs[i] = e.errs[i]
	}
	return errs
}

// GroupByURL groups URLErrors by their URL.
type GroupedErrors interface {
	ParseError
	ByURL() map[string][]URLError
}

type groupedErrors struct {
	parseErrors
}

func (e *groupedErrors) ByURL() map[string][]URLError {
	grouped := make(map[string][]URLError, len(e.errs))
	for _, ue := range e.errs {
		grouped[ue.URL()] = append(grouped[ue.URL()], ue)
	}
	return grouped
}
