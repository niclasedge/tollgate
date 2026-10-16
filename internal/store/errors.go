package store

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"modernc.org/sqlite"
)

var (
	// ErrNotFound is returned when a row or a referenced row does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrConflict is returned on unique-constraint violations.
	ErrConflict = errors.New("store: conflict")
	// ErrInvalid is returned when an argument fails validation. Messages name
	// the field but never quote the offending value.
	ErrInvalid = errors.New("store: invalid argument")
)

const (
	// MaxDiffBytes caps the redacted diff stored per stage.
	MaxDiffBytes = 64 << 10
	maxKeyHint   = 16
	maxComment   = 1024
	sha256Len    = 32
	unknownIdent = "unknown"
	maxList      = 1000
)

var (
	identRe = regexp.MustCompile(`^[A-Za-z0-9_.:/-]{1,64}$`)
	// modelRe additionally allows brackets for context-window suffixes such
	// as "claude-opus-4-1[1m]".
	modelRe = regexp.MustCompile(`^[A-Za-z0-9_.:/\[\]-]{1,64}$`)
	hexRe   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// placeholderRe is the placeholder grammar of ADR-0006; nothing else may
	// reach the placeholder field of a match.
	placeholderRe = regexp.MustCompile(`^<redacted:[a-z][a-z0-9_]{0,31}_[1-9][0-9]{0,5}>$`)
)

// ValidModel reports whether m is a valid model identifier (as stored in
// traces.model).
func ValidModel(m string) bool { return modelRe.MatchString(m) }

func invalid(field string) error { return fmt.Errorf("%w: %s", ErrInvalid, field) }

func checkIdent(field, v string) error {
	if !identRe.MatchString(v) {
		return invalid(field)
	}
	return nil
}

// normIdent returns v if it is a plausible identifier for the given pattern,
// otherwise "unknown". It is used for untrusted client input (model,
// endpoint): a bad value must neither reach the database nor drop the trace.
func normIdent(re *regexp.Regexp, v string) string {
	if re.MatchString(v) {
		return v
	}
	return unknownIdent
}

func checkOptIdent(field, v string) error {
	if v == "" {
		return nil
	}
	return checkIdent(field, v)
}

func checkHash(field string, h []byte) error {
	if len(h) != sha256Len {
		return invalid(field)
	}
	return nil
}

func checkHex(field, h string) error {
	if !hexRe.MatchString(h) {
		return invalid(field)
	}
	return nil
}

func hexOf(b []byte) string { return hex.EncodeToString(b) }

// mapErr translates SQLite constraint errors into the package errors.
func mapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case 2067, 1555: // SQLITE_CONSTRAINT_UNIQUE, SQLITE_CONSTRAINT_PRIMARYKEY
			return fmt.Errorf("store: %s: %w", op, ErrConflict)
		case 787: // SQLITE_CONSTRAINT_FOREIGNKEY
			return fmt.Errorf("store: %s: referenced row: %w", op, ErrNotFound)
		}
	}
	return fmt.Errorf("store: %s: %w", op, err)
}

// maxMaskBytes bounds the mask of a match (issue #112).
const maxMaskBytes = 64

// validMask accepts an empty mask or a short, printable mask with at least
// one bullet. A plaintext value passed by mistake has no bullet and is
// rejected.
func validMask(m string) bool {
	if m == "" {
		return true
	}
	if len(m) > maxMaskBytes || !utf8.ValidString(m) || !strings.Contains(m, "•") {
		return false
	}
	return strings.IndexFunc(m, unicode.IsControl) < 0
}
