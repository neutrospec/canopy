package embed

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// protoFields walks protobuf fields by offset. Large inline tensor payloads
// are skipped with ReaderAt rather than read or hashed during readiness.
type protoField struct {
	number     int
	wire       byte
	start, end int64
	value      uint64
}

func protoFields(r io.ReaderAt, start, end int64, visit func(protoField) error) error {
	pos := start
	readVarint := func() (uint64, error) {
		var v uint64
		var b [1]byte
		for n := uint(0); n < 70; n += 7 {
			if pos >= end {
				return 0, io.ErrUnexpectedEOF
			}
			if _, err := r.ReadAt(b[:], pos); err != nil {
				return 0, err
			}
			pos++
			if n == 63 && b[0] > 1 {
				return 0, fmt.Errorf("protobuf varint overflow")
			}
			v |= uint64(b[0]&127) << n
			if b[0] < 128 {
				return v, nil
			}
		}
		return 0, fmt.Errorf("protobuf varint overflow")
	}
	for pos < end {
		tag, err := readVarint()
		if err != nil {
			return err
		}
		f := protoField{number: int(tag >> 3), wire: byte(tag & 7)}
		if f.number == 0 {
			return fmt.Errorf("invalid protobuf field")
		}
		switch f.wire {
		case 0:
			f.value, err = readVarint()
		case 1:
			f.start = pos
			pos += 8
			f.end = pos
		case 2:
			var n uint64
			n, err = readVarint()
			if n > uint64(end-pos) {
				return io.ErrUnexpectedEOF
			}
			f.start = pos
			pos += int64(n)
			f.end = pos
		case 5:
			f.start = pos
			pos += 4
			f.end = pos
		default:
			return fmt.Errorf("unsupported protobuf wire %d", f.wire)
		}
		if err != nil {
			return err
		}
		if pos > end {
			return io.ErrUnexpectedEOF
		}
		if err = visit(f); err != nil {
			return err
		}
	}
	return nil
}

// externalFiles inspects main-graph tensor declarations, including constant
// attributes and nested subgraphs. It does not read weight tensor contents.
func externalFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, fmt.Errorf("model.onnx must be a nonempty regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, fmt.Errorf("model.onnx must be a nonempty regular file")
	}
	refs := map[string]bool{}
	graphFound := false
	var graph func(int64, int64, int) error
	tensor := func(start, end int64) error {
		external := false
		location := ""
		err := protoFields(f, start, end, func(v protoField) error {
			if v.number == 14 && v.wire == 0 {
				external = v.value == 1
			}
			if v.number == 13 && v.wire == 2 {
				key, value := "", ""
				if err := protoFields(f, v.start, v.end, func(e protoField) error {
					if e.wire != 2 || e.end-e.start > 4096 {
						return fmt.Errorf("invalid external tensor metadata")
					}
					b := make([]byte, e.end-e.start)
					if _, err := f.ReadAt(b, e.start); err != nil {
						return err
					}
					if e.number == 1 {
						key = string(b)
					}
					if e.number == 2 {
						value = string(b)
					}
					return nil
				}); err != nil {
					return err
				}
				if key == "location" {
					location = value
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if external || location != "" {
			if location == "" || filepath.IsAbs(location) || filepath.Clean(location) == ".." || len(location) > 4096 {
				return fmt.Errorf("unsafe or missing ONNX external data location")
			}
			clean := filepath.ToSlash(filepath.Clean(location))
			if len(clean) >= 3 && clean[:3] == "../" {
				return fmt.Errorf("external data escapes bundle")
			}
			refs[clean] = true
		}
		return nil
	}
	graph = func(start, end int64, depth int) error {
		if depth > 32 {
			return fmt.Errorf("ONNX graph nesting too deep")
		}
		return protoFields(f, start, end, func(v protoField) error {
			if v.wire != 2 {
				return nil
			}
			if v.number == 5 {
				return tensor(v.start, v.end)
			}
			if v.number == 1 {
				return protoFields(f, v.start, v.end, func(node protoField) error {
					if node.number != 5 || node.wire != 2 {
						return nil
					}
					return protoFields(f, node.start, node.end, func(attr protoField) error {
						if attr.wire != 2 {
							return nil
						}
						switch attr.number {
						case 5, 10:
							return tensor(attr.start, attr.end)
						case 6, 11:
							return graph(attr.start, attr.end, depth+1)
						}
						return nil
					})
				})
			}
			return nil
		})
	}
	err = protoFields(f, 0, info.Size(), func(v protoField) error {
		if v.number == 7 && v.wire == 2 {
			graphFound = true
			return graph(v.start, v.end, 0)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid ONNX structure: %w", err)
	}
	if !graphFound {
		return nil, fmt.Errorf("ONNX graph missing")
	}
	names := make([]string, 0, len(refs))
	for n := range refs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}
