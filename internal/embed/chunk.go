package embed

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Chunk is a unit of text to be embedded.
type Chunk struct {
	Seq  int
	Text string
	Hash string // sha256 of Text; lets reindex skip unchanged chunks
}

// targetRunes approximates the old system's 400-token segments:
// bge-m3 averages roughly 1.5-3 runes/token on mixed Korean/English.
const (
	targetRunes = 1200
	maxRunes    = 1600
)

// Split cuts a page into chunks along paragraph boundaries. The page
// title is prepended to every chunk so retrieval keeps page context.
func Split(title, body string) []Chunk {
	paras := strings.Split(body, "\n\n")
	var chunks []Chunk
	var cur strings.Builder
	curLen := 0

	flush := func() {
		text := strings.TrimSpace(cur.String())
		cur.Reset()
		curLen = 0
		if text == "" {
			return
		}
		if title != "" {
			text = title + "\n\n" + text
		}
		sum := sha256.Sum256([]byte(text))
		chunks = append(chunks, Chunk{
			Seq:  len(chunks),
			Text: text,
			Hash: hex.EncodeToString(sum[:8]),
		})
	}

	for _, para := range paras {
		p := strings.TrimSpace(para)
		if p == "" {
			continue
		}
		n := len([]rune(p))
		// Oversized single paragraph: hard-split by runes.
		if n > maxRunes {
			flush()
			runes := []rune(p)
			for start := 0; start < len(runes); start += targetRunes {
				end := min(start+targetRunes, len(runes))
				cur.WriteString(string(runes[start:end]))
				curLen = end - start
				flush()
			}
			continue
		}
		if curLen > 0 && curLen+n > targetRunes {
			flush()
		}
		if curLen > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
		curLen += n
	}
	flush()
	return chunks
}
