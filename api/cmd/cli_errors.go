package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type cliExitError struct {
	err      error
	code     int
	reported bool
}

func (e *cliExitError) Error() string { return e.err.Error() }
func (e *cliExitError) Unwrap() error { return e.err }

type cliErrorBody struct {
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	CommandPath []string       `json:"command_path,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
}

// ExitCode returns the process exit code for a command error.
func ExitCode(err error) int {
	var exitError *cliExitError
	if errors.As(err, &exitError) && exitError.code != 0 {
		return exitError.code
	}
	return 1
}

// ErrorWasReported reports whether Run already wrote a structured error.
func ErrorWasReported(err error) bool {
	var exitError *cliExitError
	return errors.As(err, &exitError) && exitError.reported
}

func reportCLIError(args []string, err error) error {
	if err == nil {
		return nil
	}
	if !cliWantsJSON(args) {
		return err
	}
	body := cliErrorBody{Code: "command_failed", Message: err.Error(), CommandPath: cliCommandWords(args)}
	var serviceError *longhouseServiceError
	if errors.As(err, &serviceError) {
		body.Code = "service_error"
		body.Details = map[string]any{"http_status": serviceError.Status, "service_code": serviceError.Code}
	} else if strings.Contains(strings.ToLower(err.Error()), "signed in") || strings.Contains(strings.ToLower(err.Error()), "session") {
		body.Code = "authentication_required"
	} else if strings.Contains(strings.ToLower(err.Error()), "required") || strings.Contains(strings.ToLower(err.Error()), "unknown") {
		body.Code = "invalid_request"
	}
	if encodeErr := writeCLIJSON(os.Stdout, cliResultEnvelope{SchemaVersion: structuredHelpVersion, OK: false, Error: body}); encodeErr != nil {
		return fmt.Errorf("%w; write JSON error: %v", err, encodeErr)
	}
	return &cliExitError{err: err, code: 1, reported: true}
}

func cliWantsJSON(args []string) bool {
	for i, arg := range args {
		if arg == "--json" || arg == "--output=json" {
			return true
		}
		if arg == "--output" && i+1 < len(args) && args[i+1] == "json" {
			return true
		}
	}
	return false
}

func cliCommandWords(args []string) []string {
	parsed, err := parseCLIArguments(args)
	if err != nil {
		return nil
	}
	return parsed.Words
}

func cliCanceled(message string) error {
	return &cliExitError{err: errors.New(message), code: 130}
}
