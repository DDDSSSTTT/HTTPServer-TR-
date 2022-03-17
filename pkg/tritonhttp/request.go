package tritonhttp

import (
	"bufio"
	"fmt"
	"net/url"
	"strings"
)

type Request struct {
	Method string // e.g. "GET"
	URL    string // e.g. "/path/to/a/file"
	Proto  string // e.g. "HTTP/1.1"

	// Header stores misc headers excluding "Host" and "Connection",
	// which are stored in special fields below.
	// Header keys are case-incensitive, and should be stored
	// in the canonical format in this map.
	Header map[string]string

	Host  string // determine from the "Host" header
	Close bool   // determine from the "Connection" header
}

// ReadRequest tries to read the next valid request from br.
//
// If it succeeds, it returns the valid request read. In this case,
// bytesReceived should be true, and err should be nil.
//
// If an error occurs during the reading, it returns the error,
// and a nil request. In this case, bytesReceived indicates whether or not
// some bytes are received before the error occurs. This is useful to determine
// the timeout with partial request received condition.
func parseRequestLine(line string) ([]string, error) {
	fields := strings.SplitN(line, " ", 3)
	if len(fields) != 3 {
		return fields, fmt.Errorf("could not parse the request line, got fields %v", fields)
	}
	return fields, nil
}
func badStringError(what, val string) error {
	return fmt.Errorf("%s %q", what, val)
}
func validMethod(method string) bool {
	return method == "GET"
}
func validProto(proto string) bool {
	return proto == "HTTP/1.1"
}

func ReadRequest(br *bufio.Reader) (req *Request, bytesReceived bool, err error) {

	req = &Request{}
	// Read start line
	line, err := ReadLine(br)
	if err != nil {
		return nil, false, err
	}
	if line == "" {
		return nil, false, badStringError("Empty start line", line)
	}
	firstline, err := parseRequestLine(line)
	if len(firstline) < 1 {
		return nil, false, badStringError("empty request", "")
	}
	req.Method = firstline[0]
	req.URL = firstline[1]
	req.Proto = firstline[2]
	if req.Proto == "" {
		return nil, true, badStringError("missing proto", req.Proto)
	}
	if err != nil {
		return nil, true, badStringError("malformed start line", line)
	}
	// Check required headers
	if !validMethod(req.Method) {
		return nil, true, badStringError("invalid method", req.Method)
	}
	u, err := url.ParseRequestURI(req.URL)
	if err != nil {
		return nil, true, badStringError("malformed URL", u.String())
	}
	if !validProto(req.Proto) {
		return nil, true, badStringError("invalid proto", req.Proto)
	}
	// TODO:Maybe I should write a URL Checker here.
	// Read headers
	req.Header = make(map[string]string)
	for {
		line, err := ReadLine(br)
		if err != nil {
			if req.Host == "" {
				return nil, true, badStringError("Missing Host", "")
			}
			//EOF error
			return nil, true, err
		}
		if line == "" {
			// This marks header end
			break
		}
		s := strings.SplitN(line, ": ", 2)
		switch {
		// Handle special headers
		case s[0] == "Host":
			req.Host = strings.TrimSpace(s[1])
		case s[0] == "Connection":
			req.Close = s[1] == "close"
		default:
			req.Header[CanonicalHeaderKey(s[0])] = strings.TrimSpace(s[1])
		}

	}

	return req, true, err
}
