package validation

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
)

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var portRegex = regexp.MustCompile(`^([1-9][0-9]{0,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])$`)
var urlRegex = regexp.MustCompile(`^(https?:\/\/)([\da-z\.\-]+)\.([a-z\.]{2,6})([\/\w \.\-]*)*\/?$`)
var nameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_\.]*[a-zA-Z0-9]$`)

func ValidateDomain(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return errors.New("domain is required")
	}
	if len(domain) > 253 {
		return errors.New("domain exceeds maximum length of 253 characters")
	}
	if !domainRegex.MatchString(domain) {
		return errors.New("invalid domain format")
	}
	return nil
}

func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("email is required")
	}
	if len(email) > 254 {
		return errors.New("email exceeds maximum length of 254 characters")
	}
	if !emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}
	_, err := mail.ParseAddress(email)
	if err != nil {
		return errors.New("invalid email address")
	}
	return nil
}

func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

func ValidatePath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path is required")
	}
	if !strings.HasPrefix(path, "/") {
		return errors.New("path must start with /")
	}
	if len(path) > 4096 {
		return errors.New("path exceeds maximum length of 4096 characters")
	}
	return nil
}

func ValidateURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return errors.New("URL is required")
	}
	if !urlRegex.MatchString(url) {
		return errors.New("invalid URL format")
	}
	return nil
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) < 1 || len(name) > 100 {
		return errors.New("name must be between 1 and 100 characters")
	}
	if !nameRegex.MatchString(name) {
		return errors.New("name can only contain letters, numbers, hyphens, underscores, and dots")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasNumber, hasSpecial bool
	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasNumber = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", c):
			hasSpecial = true
		}
	}
	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return errors.New("password must contain at least one number")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}
	return nil
}
