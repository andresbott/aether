// internal/unidecode/unidecode.go
package unidecode

import (
	"strings"

	udecode "github.com/rainycape/unidecode"
)

func Normalize(s string) string {
	return strings.TrimSpace(strings.ToLower(udecode.Unidecode(s)))
}
