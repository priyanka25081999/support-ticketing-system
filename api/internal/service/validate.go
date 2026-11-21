package service

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"ticketing/api/internal/apperr"
)

// Input limits shared by validation helpers.
const (
	minPasswordLen    = 8
	maxPasswordLen    = 72 // bcrypt ignores bytes beyond 72
	maxNameLen        = 100
	minSubjectLen     = 3
	maxSubjectLen     = 200
	maxDescriptionLen = 5000
	maxMessageLen     = 5000
	maxLinksPerCreate = 10
	maxNotifyEmails   = 10
)

// ParseEmailList splits a ";"-separated list, trims, lowercases, de-duplicates
// and validates each address. At least one address is required.
func ParseEmailList(raw string) ([]string, error) {
	var emails []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ";") {
		email := normalizeEmail(part)
		if email == "" || seen[email] {
			continue
		}
		if err := validateEmail(email); err != nil {
			return nil, apperr.Validation(fmt.Sprintf("Invalid email address: %q", strings.TrimSpace(part)))
		}
		seen[email] = true
		emails = append(emails, email)
	}
	if len(emails) == 0 {
		return nil, apperr.Validation("At least one notification email is required")
	}
	if len(emails) > maxNotifyEmails {
		return nil, apperr.Validation(fmt.Sprintf("At most %d notification emails are allowed", maxNotifyEmails))
	}
	return emails, nil
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func validateEmail(email string) error {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 254 {
		return apperr.Validation("Please enter a valid email address")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen || len(password) > maxPasswordLen {
		return apperr.Validation(fmt.Sprintf("Password must be %d-%d characters", minPasswordLen, maxPasswordLen))
	}
	return nil
}

// requireLength trims value and checks its length in characters.
func requireLength(field, value string, minLen, maxLen int) (string, error) {
	value = strings.TrimSpace(value)
	n := utf8.RuneCountInString(value)
	if n < minLen || n > maxLen {
		return "", apperr.Validation(fmt.Sprintf("%s must be %d-%d characters", field, minLen, maxLen))
	}
	return value, nil
}
