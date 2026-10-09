package webui

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type hostPolicy struct {
	port           string
	explicit       map[string]bool
	allowLiteralIP bool
}

func authority(raw string) (host, port string, err error) {
	if raw == "" || strings.ContainsAny(raw, "/\\@#? \t\r\n") {
		return "", "", fmt.Errorf("invalid Host")
	}
	host, port, err = net.SplitHostPort(raw)
	if err != nil {
		host = strings.Trim(raw, "[]")
		port = ""
		if strings.Contains(host, ":") && net.ParseIP(host) == nil {
			return "", "", fmt.Errorf("invalid Host")
		}
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return "", "", fmt.Errorf("empty Host")
	}
	if port != "" {
		if n, e := strconv.Atoi(port); e != nil || n < 1 || n > 65535 {
			return "", "", fmt.Errorf("invalid Host port")
		}
	}
	return host, port, nil
}

func hostKey(host, port string) string { return host + "\x00" + port }

// ConfigureListener binds accepted authorities to the real listener. Extra
// authorities are explicit proxy/domain choices; forwarded headers are ignored.
// Configure once before serving requests. NewServer alone accepts loopback only.
func (s *Server) ConfigureListener(addr string, extra []string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	policy := &hostPolicy{port: port, explicit: map[string]bool{}}
	ip := net.ParseIP(host)
	policy.allowLiteralIP = host == "" || (ip != nil && ip.IsUnspecified())
	if host != "" && !policy.allowLiteralIP {
		policy.explicit[hostKey(strings.ToLower(host), port)] = true
	}
	for _, raw := range extra {
		h, p, err := authority(raw)
		if err != nil {
			return fmt.Errorf("allow-host %q: %w", raw, err)
		}
		policy.explicit[hostKey(h, p)] = true
	}
	s.hosts = policy
	return nil
}

func (s *Server) hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, p, err := authority(r.Host)
		if err != nil {
			http.Error(w, "Host rejected", http.StatusForbidden)
			return
		}
		policy := s.hosts
		allowed := false
		if policy != nil && policy.explicit[hostKey(h, p)] {
			allowed = true
		}
		portOK := policy == nil || policy.port == "" || p == policy.port
		ip := net.ParseIP(h)
		if portOK && (h == "localhost" || (ip != nil && ip.IsLoopback())) {
			allowed = true
		}
		if policy != nil && policy.allowLiteralIP && portOK && ip != nil {
			allowed = true
		}
		if !allowed {
			http.Error(w, "Host rejected; configure --allow-host for a trusted domain or proxy", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func bodyLimit(path string) int64 {
	switch {
	case strings.HasPrefix(path, "/edit/"):
		return 1 << 20
	case strings.HasPrefix(path, "/task/"):
		return 64 << 10
	case path == "/login" || path == "/setup":
		return 8 << 10
	default:
		return 16 << 10
	}
}

func (s *Server) requestLimits(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := url.ParseQuery(r.URL.RawQuery); err != nil {
			http.Error(w, "invalid query encoding", http.StatusBadRequest)
			return
		}
		limit := bodyLimit(r.URL.Path)
		if r.ContentLength > limit {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			var err error
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				err = r.ParseMultipartForm(64 << 10)
				if r.MultipartForm != nil {
					defer r.MultipartForm.RemoveAll()
				}
				if err == nil && r.MultipartForm != nil && len(r.MultipartForm.File) > 0 {
					http.Error(w, "file uploads are not supported", http.StatusBadRequest)
					return
				}
			} else {
				err = r.ParseForm()
			}
			if err != nil {
				var max *http.MaxBytesError
				status := http.StatusBadRequest
				if errors.As(err, &max) {
					status = http.StatusRequestEntityTooLarge
				}
				http.Error(w, "invalid or oversized form", status)
				return
			}
		}
		// ParseForm leaves unsupported content types and multipart epilogues
		// unread. Consume the remainder through the same cumulative cap before
		// any handler can apply a mutation, including chunked requests.
		if r.Body != nil {
			if _, err := io.Copy(io.Discard, r.Body); err != nil {
				status := http.StatusBadRequest
				var max *http.MaxBytesError
				if errors.As(err, &max) {
					status = http.StatusRequestEntityTooLarge
				}
				http.Error(w, "invalid or oversized body", status)
				return
			}
		}
		if r.URL.Path == "/search" || r.URL.Path == "/api/search" {
			select {
			case s.searchSlots <- struct{}{}:
				defer func() { <-s.searchSlots }()
			default:
				w.Header().Set("Retry-After", "1")
				http.Error(w, "search queue busy", http.StatusServiceUnavailable)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
