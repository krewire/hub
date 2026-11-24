package redis

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// EncodeRESP serializes a command and its string arguments into standard RESP format.
func EncodeRESP(args ...string) []byte {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		buf.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}
	return buf.Bytes()
}

// ReadRESP decodes a single RESP message from a buffered reader.
func ReadRESP(r *bufio.Reader) (interface{}, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return nil, fmt.Errorf("redis: empty resp response line")
	}

	prefix := line[0]
	payload := line[1:]

	switch prefix {
	case '+': // Simple string
		return payload, nil
	case '-': // Error
		return nil, fmt.Errorf("redis error: %s", payload)
	case ':': // Integer
		return strconv.ParseInt(payload, 10, 64)
	case '$': // Bulk string
		length, err := strconv.Atoi(payload)
		if err != nil {
			return nil, err
		}
		if length == -1 {
			return nil, nil // Null bulk string
		}
		buf := make([]byte, length+2) // data + \r\n
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return string(buf[:length]), nil
	case '*': // Array
		count, err := strconv.Atoi(payload)
		if err != nil {
			return nil, err
		}
		if count == -1 {
			return nil, nil
		}
		items := make([]interface{}, count)
		for i := 0; i < count; i++ {
			item, err := ReadRESP(r)
			if err != nil {
				return nil, err
			}
			items[i] = item
		}
		return items, nil
	default:
		return nil, fmt.Errorf("redis: unsupported resp marker: %c", prefix)
	}
}
