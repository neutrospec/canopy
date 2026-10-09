package embed

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// Chunk keeps evidence byte ranges separate from model input. Offsets address
// the parsed page body, including its original whitespace.
type Chunk struct {
	Seq        int
	Text       string // verbatim body[Start:End]
	Input      string // title plus normalized text; only for embedding
	Hash       string
	Start, End int
}

const (
	TargetRunes          = 1200
	MaxRunes             = 1600
	ChunkerVersion       = "raw-byte-ranges-v1/paragraph-runes/1200/1600"
	NormalizationVersion = "paragraph-trim-v1/title-prefix"
)

// Split retains contiguous raw ranges; only the model input is normalized.
func Split(title, body string) []Chunk {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	var chunks []Chunk
	emit := func(start, end int) {
		raw := body[start:end]
		if strings.TrimSpace(raw) == "" {
			return
		}
		var parts []string
		for _, p := range strings.Split(raw, "\n\n") {
			if p = strings.TrimSpace(p); p != "" {
				parts = append(parts, p)
			}
		}
		input := strings.Join(parts, "\n\n")
		if title != "" {
			input = title + "\n\n" + input
		}
		sum := sha256.Sum256([]byte(input))
		chunks = append(chunks, Chunk{Seq: len(chunks), Text: raw, Input: input, Hash: hex.EncodeToString(sum[:]), Start: start, End: end})
	}
	start := 0
	for start < len(body) {
		end, n, lastBoundary := start, 0, -1
		for end < len(body) && n < MaxRunes {
			_, size := utf8.DecodeRuneInString(body[end:])
			end += size
			n++
			if end >= 2 && body[end-2:end] == "\n\n" {
				lastBoundary = end
				if n >= TargetRunes {
					break
				}
			}
			if n >= TargetRunes && lastBoundary > start {
				end = lastBoundary
				break
			}
		}
		if end < len(body) && lastBoundary <= start {
			end = start
			for n = 0; end < len(body) && n < TargetRunes; n++ {
				_, size := utf8.DecodeRuneInString(body[end:])
				end += size
			}
		}
		emit(start, end)
		start = end
	}
	return chunks
}
