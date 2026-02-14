package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Response is the standard JSON response structure
type Response struct {
	Success   bool        `json:"success"`
	Operation string      `json:"operation,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
}

// ErrorInfo contains structured error information
type ErrorInfo struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Formatter handles output formatting (JSON or human-friendly)
type Formatter struct {
	JSONMode bool
	Writer   io.Writer
}

// NewFormatter creates a formatter instance
func NewFormatter(jsonMode bool) *Formatter {
	return &Formatter{
		JSONMode: jsonMode,
		Writer:   os.Stdout,
	}
}

// Success outputs a success response
func (f *Formatter) Success(operation string, data interface{}, message string) error {
	if f.JSONMode {
		return f.outputJSON(Response{
			Success:   true,
			Operation: operation,
			Data:      data,
			Message:   message,
		})
	}
	// Human-friendly output with checkmark
	fmt.Fprintf(f.Writer, "✓ %s\n", message)
	return nil
}

// Error outputs an error response
func (f *Formatter) Error(code, message string, details interface{}) error {
	if f.JSONMode {
		return f.outputJSON(Response{
			Success: false,
			Error: &ErrorInfo{
				Code:    code,
				Message: message,
				Details: details,
			},
		})
	}
	// Human-friendly error
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	return nil
}

// outputJSON writes JSON response
func (f *Formatter) outputJSON(resp Response) error {
	encoder := json.NewEncoder(f.Writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resp)
}
