package wiki

import (
	"regexp"
	"strings"
)

var validFilenameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*\.md$`)

// ValidFilename reports whether a page filename follows the schema:
// English only, lowercase, hyphens instead of spaces.
func ValidFilename(name string) bool {
	return validFilenameRe.MatchString(name)
}

var slugStripRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify derives a filename slug from a title. Returns "" when the
// title has no ASCII content (e.g. pure Korean) — callers must then
// require an explicit --slug from the user instead of guessing.
func Slugify(title string) string {
	s := strings.ToLower(title)
	s = slugStripRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if !regexp.MustCompile(`[a-z]`).MatchString(s) {
		return ""
	}
	return s
}
