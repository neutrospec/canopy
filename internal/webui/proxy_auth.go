package webui

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ReadTokenFile loads a single 256-bit hex token. Errors never include its contents.
func ReadTokenFile(path, wikiRoot string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("auth token file must be an absolute path")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve auth token file: %w", err)
	}
	if wikiRoot != "" {
		root, err := filepath.EvalSymlinks(wikiRoot)
		if err != nil {
			return "", fmt.Errorf("cannot resolve wiki root: %w", err)
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return "", fmt.Errorf("auth token file must be outside the wiki")
		}
	}
	for dir := filepath.Dir(resolved); ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return "", fmt.Errorf("auth token file must be outside Git repositories")
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("cannot stat auth token file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return "", fmt.Errorf("auth token file must be a regular file with mode 0600")
	}
	f, err := os.Open(resolved)
	if err != nil {
		return "", fmt.Errorf("cannot open auth token file: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 67))
	if err != nil {
		return "", fmt.Errorf("cannot read auth token file")
	}
	token := string(b)
	if strings.HasSuffix(token, "\n") {
		token = strings.TrimSuffix(strings.TrimSuffix(token, "\n"), "\r")
	}
	if len(token) != 64 {
		return "", fmt.Errorf("auth token file must contain one 64-character hex token")
	}
	if _, err := hex.DecodeString(token); err != nil {
		return "", fmt.Errorf("auth token file must contain one 64-character hex token")
	}
	return token, nil
}

// EnableTokenAuth preserves existing accounts but does not bootstrap a new one.
func (s *Server) EnableTokenAuth(token string) error {
	a, err := loadAuthStore(webAuthPath())
	if err != nil {
		return err
	}
	s.auth = a
	s.authRequired = true
	digest := sha256.Sum256([]byte(token))
	s.tokenDigest = &digest
	return nil
}

func (s *Server) bearerOK(r *http.Request) bool {
	headers := r.Header.Values("Authorization")
	if len(headers) != 1 || s.tokenDigest == nil {
		return false
	}
	scheme, token, found := strings.Cut(headers[0], " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" || strings.ContainsAny(token, " \t\r\n,") {
		return false
	}
	digest := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(digest[:], s.tokenDigest[:]) == 1
}

func parseOrigin(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(raw, "*\\#") {
		return nil, fmt.Errorf("origin must be a single HTTP(S) origin without path, query or fragment")
	}
	if _, _, err := authority(u.Host); err != nil {
		return nil, fmt.Errorf("invalid origin authority")
	}
	return u, nil
}

func (s *Server) ConfigurePublicOrigin(raw string) error {
	if raw == "" {
		s.publicOrigin = ""
		return nil
	}
	u, err := parseOrigin(raw)
	if err != nil || u.Scheme != "https" {
		return fmt.Errorf("public origin must be an exact HTTPS origin without path, query or fragment")
	}
	s.publicOrigin = raw
	return nil
}

func (s *Server) originOK(r *http.Request) bool {
	origins := r.Header.Values("Origin")
	if len(origins) == 0 {
		return true
	} // Existing non-browser clients.
	if len(origins) != 1 {
		return false
	}
	raw := origins[0]
	u, err := parseOrigin(raw)
	if err != nil {
		return false
	}
	if s.publicOrigin != "" && raw == s.publicOrigin {
		return true
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return u.Scheme == scheme && strings.EqualFold(u.Host, r.Host)
}

func authenticationRequired(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="canopy"`)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = io.WriteString(w, "{\"error\":\"authentication_required\"}\n")
}
