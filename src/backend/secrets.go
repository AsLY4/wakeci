package main

import (
	"regexp"
	"strings"
)

// secretsRegex matches exactly one `{{ secrets.NAME }}` reference at a time.
var secretsRegex = regexp.MustCompile(`{{\s*secrets\.([A-Za-z0-9_]+)\s*}}`)

const redactedSecret = "***REDACTED***"

// injectSecrets replaces every `{{ secrets.NAME }}` reference in str with the
// named secret's value (or the literal "<no value>" if NAME isn't
// configured, matching text/template's own missing-key rendering).
//
// This deliberately does its own targeted substitution instead of running
// str through text/template: executing it as a template against
// Config.secrets (the previous approach) let a single {{ secrets.X }}
// reference - required only to pass the "does this string use secrets"
// check - unlock arbitrary template actions elsewhere in the same string,
// including referencing a *different* secret than the one named, or a
// {{ range $k, $v := . }} that dumped every configured secret at once. A
// plain regex substitution can only ever resolve the one named key.
func injectSecrets(str string) string {
	return secretsRegex.ReplaceAllStringFunc(str, func(match string) string {
		key := secretsRegex.FindStringSubmatch(match)[1]
		if value, ok := Config.secrets[key]; ok {
			return value
		}
		return "<no value>"
	})
}

// redactSecrets is a function that redacts secrets from the string (build logs, etc.)
func redactSecrets(str string) string {
	for _, value := range Config.secrets {
		str = strings.ReplaceAll(str, value, redactedSecret)
	}
	return str
}
