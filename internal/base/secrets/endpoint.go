package secrets

import (
	"errors"
	"net/url"
	"regexp"
	"strings"

	"reasonix/internal/base/shellparse"
)

const EndpointRedacted = "<redacted>"

var endpointPattern = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s<>]+`)
var endpointSchemePattern = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*://`)

func CredentialKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if EnvKeySensitive(key) {
		return true
	}
	for _, part := range strings.FieldsFunc(key, func(r rune) bool { return r == '_' || r == '-' || r == '.' }) {
		switch part {
		case "auth", "authorization", "bearer", "credential", "credentials", "cookie", "apikey", "signature", "sig":
			return true
		}
	}
	return false
}

func EndpointQueryKey(key string) bool {
	return strings.EqualFold(strings.TrimSpace(key), "key") || CredentialKey(key)
}

// RedactEndpoint fails closed because a partial parse cannot establish which bytes are credentials.
func RedactEndpoint(raw string) string {
	if strings.TrimSpace(raw) == "" || raw == EndpointRedacted {
		return raw
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Scheme == "" || u.Opaque != "" {
		return EndpointRedacted
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return EndpointRedacted
	}
	changed := u.User != nil
	u.User = nil
	segments := strings.Split(u.Path, "/")
	for i, segment := range segments {
		if EndpointQueryKey(segment) && i+1 < len(segments) {
			u.Path = strings.Join(segments[:i+1], "/") + "/" + EndpointRedacted
			u.RawPath = ""
			changed = true
			break
		}
	}
	if redactEndpointQuery(query) {
		u.RawQuery = query.Encode()
		changed = true
	}
	if u.Fragment != "" {
		if !strings.Contains(u.Fragment, "=") {
			u.Fragment, u.RawFragment = EndpointRedacted, ""
			changed = true
		} else {
			fragment, err := url.ParseQuery(u.Fragment)
			if err != nil {
				return EndpointRedacted
			}
			if redactEndpointQuery(fragment) {
				u.Fragment = fragment.Encode()
				u.RawFragment = ""
				changed = true
			}
		}
	}
	if !changed {
		return raw
	}
	return u.String()
}

func redactEndpointQuery(query url.Values) bool {
	changed := false
	for key, values := range query {
		if !EndpointQueryKey(key) {
			continue
		}
		for i, value := range values {
			if value != "" && value != EndpointRedacted {
				values[i] = EndpointRedacted
				changed = true
			}
		}
	}
	return changed
}

func CredentialValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	key, _, assignment := strings.Cut(trimmed, "=")
	header, _, colon := strings.Cut(trimmed, ":")
	return (assignment && CredentialKey(key)) || (colon && CredentialKey(header)) || strings.HasPrefix(strings.ToLower(trimmed), "bearer ") || strings.HasPrefix(strings.ToLower(trimmed), "basic ")
}

func RedactConfigValue(key, value string) string {
	if CredentialKey(key) || CredentialValue(value) {
		return EndpointRedacted
	}
	if endpointSchemePattern.MatchString(strings.TrimSpace(value)) && !strings.ContainsAny(strings.TrimSpace(value), " \t\n\r\f") {
		return RedactEndpoint(value)
	}
	projected := endpointPattern.ReplaceAllStringFunc(value, RedactEndpoint)
	if strings.ContainsAny(projected, " \t\n") {
		command, err := shellparse.ParseStaticCommand(projected, shellparse.StaticCommandPolicy{})
		if err != nil {
			return EndpointRedacted
		}
		if len(command.Argv) > 1 {
			args := RedactArgs(command.Argv)
			for i := range args {
				if args[i] != command.Argv[i] {
					return EndpointRedacted
				}
			}
		}
	}
	return projected
}

func RedactConfigMap(fields map[string]string) map[string]string {
	if fields == nil {
		return nil
	}
	out := make(map[string]string, len(fields))
	for key, value := range fields {
		out[key] = RedactConfigValue(key, value)
	}
	return out
}

func RedactArgs(args []string) []string {
	out := append([]string(nil), args...)
	for i := 0; i < len(out); i++ {
		if len(out[i]) > 2 && (strings.HasPrefix(out[i], "-H") || strings.HasPrefix(out[i], "-e")) {
			out[i] = out[i][:2] + EndpointRedacted
			continue
		}
		if strings.ContainsAny(out[i], " \t\n") {
			projected := RedactConfigValue("", out[i])
			if projected != out[i] {
				out[i] = projected
				continue
			}
		}
		if endpointSchemePattern.MatchString(strings.TrimSpace(out[i])) {
			out[i] = RedactConfigValue("", out[i])
			continue
		}
		key, value, inline := strings.Cut(out[i], "=")
		carrier := key == "-H" || key == "--header" || key == "--headers" || key == "--env" || key == "-e"
		if carrier || (strings.HasPrefix(key, "-") && EndpointQueryKey(strings.TrimLeft(key, "-"))) {
			if inline {
				out[i] = key + "=" + EndpointRedacted
			} else if i+1 < len(out) {
				i++
				out[i] = EndpointRedacted
			}
			continue
		}
		if inline {
			out[i] = key + "=" + RedactConfigValue(key, value)
		} else {
			out[i] = RedactConfigValue("", out[i])
		}
	}
	return out
}

type diagnosticError struct{ cause error }

func (e diagnosticError) Error() string {
	var endpoint *url.Error
	if errors.As(e.cause, &endpoint) {
		copy := *endpoint
		copy.URL = RedactEndpoint(endpoint.URL)
		copy.Err = DiagnosticError(endpoint.Err)
		return copy.Error()
	}
	return RedactCredentials(e.cause.Error())
}
func (e diagnosticError) Unwrap() error { return e.cause }

// DiagnosticError preserves error identity while limiting the external display to redacted text.
func DiagnosticError(err error) error {
	if err == nil {
		return nil
	}
	return diagnosticError{cause: err}
}
