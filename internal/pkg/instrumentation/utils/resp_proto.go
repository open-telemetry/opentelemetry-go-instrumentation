// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// parseNextResp parses a RESP command from the beginning of the input string.
// Returns: (command string, number of segments, leftover string, error).
func parseNextResp(input string) (cmd string, segCount int, leftover string, err error) {
	if !strings.HasPrefix(input, "*") {
		return "", 0, input, errors.New("not a valid RESP array: missing '*'")
	}

	firstLineEnd := strings.Index(input, "\r\n")
	if firstLineEnd == -1 {
		return "", 0, input, errors.New("invalid RESP: missing CRLF after '*' line")
	}

	arrayCountStr := input[1:firstLineEnd]
	arrayCount, convErr := strconv.Atoi(arrayCountStr)
	if convErr != nil {
		return "", 0, input, fmt.Errorf("invalid array count: %w", convErr)
	}
	segCount = arrayCount

	currentPos := firstLineEnd + 2
	for i := 0; i < arrayCount; i++ {
		if currentPos >= len(input) || input[currentPos] != '$' {
			return "", 0, input, fmt.Errorf("invalid bulk string (missing '$') at index %d", i)
		}

		bulkLineEnd := strings.Index(input[currentPos:], "\r\n")
		if bulkLineEnd == -1 {
			return "", 0, input, errors.New("invalid RESP: missing CRLF in bulk length line")
		}
		bulkLineEnd += currentPos

		bulkLenStr := input[currentPos+1 : bulkLineEnd]
		bulkLen, convErr := strconv.Atoi(bulkLenStr)
		if convErr != nil {
			return "", 0, input, fmt.Errorf("invalid bulk length: %w", convErr)
		}

		// Skip "$... \r\n"
		currentPos = bulkLineEnd + 2

		if currentPos+bulkLen+2 > len(input) {
			return "", 0, input, errors.New("not enough data for bulk string content")
		}

		currentPos += bulkLen
		if input[currentPos:currentPos+2] != "\r\n" {
			return "", 0, input, errors.New("missing CRLF after bulk string data")
		}
		currentPos += 2
	}

	cmd = input[:currentPos]
	leftover = input[currentPos:]
	return cmd, segCount, leftover, nil
}

func parseSingleResp(input string) (string, error) {
	// Attempt to parse the first command from the input.
	cmd, _, _, err := parseNextResp(input)
	if err != nil {
		return "", err
	}
	return cmd, nil
}

func parsePipelineWithTotalSegs(totalSegs int, input string) (string, error) {
	if totalSegs < 0 {
		return "", errors.New("totalSegs must not be negative")
	}
	if totalSegs == 0 {
		// Parse ONLY the first valid command
		return parseSingleResp(input)
	}
	var sb strings.Builder
	parsedSoFar := 0
	rest := input

	for {
		if parsedSoFar == totalSegs {
			// We have reached the exact required segments.
			break
		} else if parsedSoFar > totalSegs {
			// We have exceeded the required segments.
			return "", fmt.Errorf("parsed segments (%d) exceed totalSegs (%d)", parsedSoFar, totalSegs)
		}

		cmd, segCount, leftover, err := parseNextResp(rest)
		if err != nil {
			// The RESP packets obtained from eBPF may not be complete.
			// Considering if the valid segments in the packet are fewer than the required segs,
			// we should retrieve as many db statements as possible.
			// When the leftover starts with *, it indicates that the remaining packet might be truncated.
			if len(leftover) > 0 && leftover[0] == '*' {
				break
			}
			return "", fmt.Errorf("cannot parse the next command: %w", err)
		}

		_, err = sb.WriteString(cmd)
		if err != nil {
			return "", fmt.Errorf("resp parse err, failed to write string: %w", err)
		}
		parsedSoFar += segCount
		rest = leftover

		// If there is no more data but we haven't reached the required segments yet, it's a failure.
		if strings.TrimSpace(rest) == "" && parsedSoFar < totalSegs {
			return "", fmt.Errorf("no more data but totalSegs not reached: need %d, got %d", totalSegs, parsedSoFar)
		}
	}

	// If we exit the loop with parsedSoFar == totalSegs, everything is correct.
	if parsedSoFar <= totalSegs {
		return sb.String(), nil
	}
	return "", fmt.Errorf("parsed segments (%d) != totalSegs (%d)", parsedSoFar, totalSegs)
}

// ParseAll parses the given RESP data and supports the following types:
//  1. *N             -- array
//  2. $N             -- bulk string
//  3. +xxx\r\n       -- simple string
//  4. -xxx\r\n       -- error message
//  5. :xxx\r\n       -- integer
//
// Returns something like []string{"set key1 value1", "GET key1", "OK", "1000", "Error message"}, etc.
func ParseRESP(segs int32, respMsg []byte) ([]string, error) {
	respData, err := parsePipelineWithTotalSegs(int(segs), unix.ByteSliceToString(respMsg))
	if err != nil {
		log.Printf("RESP parse err: %s\n", err.Error())
		return nil, err
	}
	r := bytes.NewReader([]byte(respData))
	var commands []string

	for {
		cmd, err := parseOne(r)
		if err != nil {
			if errors.Is(err, io.EOF) && cmd == "" {
				break
			}
			return nil, err
		}
		if cmd == "" {
			break
		}
		commands = append(commands, cmd)
	}
	return commands, nil
}

//	 parseOne parses a single top-level resp data:
//		*N   -- Array, corresponding to a command (e.g., set key value1)
//		$N   -- Bulk string
//		+xxx -- Simple string (e.g., +OK)
//		-xxx -- Error message
//		:xxx -- Integer
//
// Returns a readable string, or io.EOF or another error if parsing fails.
func parseOne(r *bytes.Reader) (string, error) {
	b, err := r.ReadByte()
	if err != nil {
		return "", err
	}

	switch b {
	case '*':
		count, err := readIntCRLF(r)
		if err != nil {
			return "", err
		}
		parts := make([]string, 0, count)
		for i := 0; i < count; i++ {
			bulkType, err := r.ReadByte()
			if err != nil {
				return "", err
			}
			if bulkType != '$' {
				return "", fmt.Errorf("parse error: expected '$' but got '%c'", bulkType)
			}
			strVal, err := parseBulkString(r)
			if err != nil {
				return "", err
			}
			parts = append(parts, strVal)
		}
		return strings.Join(parts, " "), nil

	case '$':
		strVal, err := parseBulkString(r)
		return strVal, err

	case '+':
		line, err := readLine(r)
		if err != nil {
			return "", err
		}
		return line, nil

	case '-':
		line, err := readLine(r)
		if err != nil {
			return "", err
		}
		return line, nil

	case ':':
		line, err := readLine(r)
		if err != nil {
			return "", err
		}
		return line, nil

	default:
		if err := r.UnreadByte(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
}

// parseBulkString parse "$N\r\n...N bytes...\r\n".
func parseBulkString(r *bytes.Reader) (string, error) {
	length, err := readIntCRLF(r)
	if err != nil {
		return "", err
	}
	if length < 0 {
		return "", nil
	}

	buf := make([]byte, length)
	n, err := io.ReadFull(r, buf)
	if err != nil {
		return "", err
	}
	if n != length {
		return "", fmt.Errorf("parse error: not enough bulk data")
	}

	if err := discardCRLF(r); err != nil {
		return "", err
	}
	return string(buf), nil
}

// readIntCRLF parses an integer string in the format "123\r\n".
func readIntCRLF(r *bytes.Reader) (int, error) {
	line, err := readLine(r)
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(line)
	if err != nil {
		return 0, fmt.Errorf("parse int error: %v", err)
	}
	return i, nil
}

// readLine reads a line until it encounters the "\r\n" sequence,
// and returns the portion of the line excluding the "\r\n".
func readLine(r *bytes.Reader) (string, error) {
	var sb strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if b == '\r' {
			// 检查下一个是否是 '\n'
			b2, err := r.ReadByte()
			if err != nil {
				return "", err
			}
			if b2 == '\n' {
				break
			}
			return "", fmt.Errorf("parse error: expected LF after CR, got '%c'", b2)
		}
		_ = sb.WriteByte(b)
	}
	return sb.String(), nil
}

// discardCRLF discard "\r\n".
func discardCRLF(r *bytes.Reader) error {
	b, err := r.ReadByte()
	if err != nil {
		return err
	}
	if b != '\r' {
		return fmt.Errorf("expected CR but got '%c'", b)
	}
	b, err = r.ReadByte()
	if err != nil {
		return err
	}
	if b != '\n' {
		return fmt.Errorf("expected LF but got '%c'", b)
	}
	return nil
}
