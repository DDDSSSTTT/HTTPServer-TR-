package tritonhttp

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
)

type Response struct {
	StatusCode int    // e.g. 200
	Proto      string // e.g. "HTTP/1.1"

	// Header stores all headers to write to the response.
	// Header keys are case-incensitive, and should be stored
	// in the canonical format in this map.
	Header map[string]string

	// Request is the valid request that leads to this response.
	// It could be nil for responses not resulting from a valid request.
	Request *Request

	// FilePath is the local path to the file to serve.
	// It could be "", which means there is no file to serve.
	FilePath string
}

var StatusDict = map[int]string{
	200: "OK",
	400: "Bad Request",
	404: "Not Found",
}

// Write writes the res to the w.
func (res *Response) Write(w io.Writer) error {
	if err := res.WriteStatusLine(w); err != nil {
		return err
	}
	if err := res.WriteSortedHeaders(w); err != nil {
		return err
	}
	if err := res.WriteBody(w); err != nil {
		return err
	}
	return nil
}

// WriteStatusLine writes the status line of res to w, including the ending "\r\n".
// For example, it could write "HTTP/1.1 200 OK\r\n".
func (res *Response) WriteStatusLine(w io.Writer) error {
	bw := bufio.NewWriter(w)
	statusLine := fmt.Sprintf("%v %v %v\r\n", res.Proto, res.StatusCode, StatusDict[res.StatusCode])
	if _, err := bw.WriteString(statusLine); err != nil {
		return err
	}

	if err := bw.Flush(); err != nil {
		return err
	}
	return nil
}

// WriteSortedHeaders writes the headers of res to w, including the ending "\r\n".
// For example, it could write "Connection: close\r\nDate: foobar\r\n\r\n".
// For HTTP, there is no need to write headers in any particular order.
// TritonHTTP requires to write in sorted order for the ease of testing.
func (res *Response) WriteSortedHeaders(w io.Writer) error {
	bw := bufio.NewWriter(w)
	this_map := res.Header
	keys := make([]string, 0, len(this_map))
	for k := range this_map {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		headline := k + ": " + this_map[k] + "\r\n"
		if _, err := bw.WriteString(headline); err != nil {
			return err
		}

		if err := bw.Flush(); err != nil {
			return err
		}
	}
	if _, err := bw.WriteString("\r\n"); err != nil {
		return err
	}

	if err := bw.Flush(); err != nil {
		return err
	}
	return nil
}

// WriteBody writes res' file content as the response body to w.
// It doesn't write anything if there is no file to serve.
func (res *Response) WriteBody(w io.Writer) error {
	bw := bufio.NewWriter(w)
	var bodyLine string
	switch {
	case res.FilePath == "":
		bodyLine = ""
	default:
		WriteFromFile(res.FilePath, bw)

	}
	if _, err := bw.WriteString(bodyLine); err != nil {
		return err
	}

	if err := bw.Flush(); err != nil {
		return err
	}
	return nil
}
func (res *Response) init() {
	res.Proto = responseProto
}
func WriteFromFile(filepath string, bw *bufio.Writer) error {
	file, err := os.Open(filepath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if len(line) == 0 && err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if _, err := bw.WriteString(line); err != nil {
			return err
		}
		if err := bw.Flush(); err != nil {
			return err
		}
	}
	return nil
}
