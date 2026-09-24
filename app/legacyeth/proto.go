package legacyeth

import (
	"encoding/binary"
	"fmt"
)

// walk iterates the fields of a protobuf message without a schema.
//
// fn receives the field number, the wire type, the raw bytes for
// length-delimited fields, and the decoded value for varints. Fixed-width fields
// are skipped: none of the legacy messages use them.
func walk(b []byte, fn func(field, wire int, val []byte, num uint64) error) error {
	i := 0
	for i < len(b) {
		key, n := binary.Uvarint(b[i:])
		if n <= 0 {
			return fmt.Errorf("bad field key at %d", i)
		}
		i += n

		field, wire := int(key>>3), int(key&7)
		switch wire {
		case 0: // varint
			v, n := binary.Uvarint(b[i:])
			if n <= 0 {
				return fmt.Errorf("bad varint at %d", i)
			}
			i += n
			if err := fn(field, wire, nil, v); err != nil {
				return err
			}
		case 2: // length-delimited
			l, n := binary.Uvarint(b[i:])
			if n <= 0 {
				return fmt.Errorf("bad length at %d", i)
			}
			i += n
			end := i + int(l)
			if l > uint64(len(b)) || end > len(b) {
				return fmt.Errorf("length %d overruns message at %d", l, i)
			}
			if err := fn(field, wire, b[i:end], 0); err != nil {
				return err
			}
			i = end
		case 1: // fixed64 — only the deprecated `size` field, ignored
			if i+8 > len(b) {
				return fmt.Errorf("truncated fixed64 at %d", i)
			}
			i += 8
		case 5: // fixed32
			if i+4 > len(b) {
				return fmt.Errorf("truncated fixed32 at %d", i)
			}
			i += 4
		default:
			return fmt.Errorf("unsupported wire type %d for field %d", wire, field)
		}
	}
	return nil
}

// unpackAny reads a google.protobuf.Any: 1 type_url, 2 value.
func unpackAny(b []byte) (string, []byte, error) {
	var url string
	var val []byte
	err := walk(b, func(field, wire int, v []byte, _ uint64) error {
		switch field {
		case 1:
			url = string(v)
		case 2:
			val = v
		}
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	if url == "" {
		return "", nil, fmt.Errorf("Any without type_url")
	}
	return url, val, nil
}
