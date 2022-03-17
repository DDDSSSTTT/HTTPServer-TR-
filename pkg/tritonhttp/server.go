package tritonhttp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	responseProto = "HTTP/1.1"

	statusOK               = 200
	statusMethodNotAllowed = 400
	statusNotFound         = 404
)

type Server struct {
	// Addr specifies the TCP address for the server to listen on,
	// in the form "host:port". It shall be passed to net.Listen()
	// during ListenAndServe().
	Addr string // e.g. ":0"

	// DocRoot specifies the path to the directory to serve static files from.
	DocRoot string
}

var mu sync.Mutex
var count int
var leaving = make(chan message)
var messages = make(chan message)

type message struct {
	text    string
	address string
}

func fatalCheck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func Check(err error) {
	if err != nil {
		log.Print(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s %s %s\n", r.Method, r.URL, r.Proto)
	for k, v := range r.Header {
		fmt.Fprintf(w, "Header[%q] = %q\n", k, v)
	}
	fmt.Fprintf(w, "Host = %q\n", r.Host)
	fmt.Fprintf(w, "RemoteAddr = %q\n", r.RemoteAddr)
	if err := r.ParseForm(); err != nil {
		log.Print(err)
	}
	for k, v := range r.Form {
		fmt.Fprintf(w, "Form[%q] = %q\n", k, v)
	}
}
func counter(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	fmt.Fprintf(w, "Count %d\n", count)
	mu.Unlock()
}

// ListenAndServe listens on the TCP network address s.Addr and then
// handles requests on incoming connections.
func (s *Server) ListenAndServe() error {
	listen, err := net.Listen("tcp", s.Addr)
	fatalCheck(err)
	log.Printf("Listening on %s\n", s.Addr)

	for {
		conn, err := listen.Accept()
		log.Println("Received a new connection")
		if err != nil {
			log.Print(err)
			continue
		}
		// Hint: call HandleConnection
		go s.HandleConnection(conn)

	}

	// http.HandleFunc("/", s.HandleConnection)
	// http.HandleFunc("/count", counter)
	//err := http.ListenAndServe(s.Addr, nil)

	return err
}

// HandleConnection reads requests from the accepted conn and handles them.
func (s *Server) HandleConnection(conn net.Conn) {

	//func (s *Server) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// panic("todo")
	// Hint: use the other methods below
	timeout := 5 * time.Second
	input := bufio.NewReader(conn)
	for {
		// Set timeout

		// Try to read next request
		if ddl_err := conn.SetReadDeadline(time.Now().Add(timeout)); ddl_err != nil {
			log.Printf("Start Deadline failed for connection %v", conn)
			//res := &Response{}
			//res.HandleBadRequest()
			//_ = res.Write(conn)
			_ = conn.Close()
			return
		}
		req, bytes_received, err := ReadRequest(input)
		// Handle EOF
		if errors.Is(err, io.EOF) {
			res := &Response{}
			res.HandleBadRequest()
			_ = res.Write(conn)
			log.Printf("Connection closed by %v", conn.RemoteAddr())
			_ = conn.Close()
			return
		}
		// Handle timeout
		// timeout in this application means the otherside stop to send any further information
		// Maybe connection lost?
		if err, ok := err.(net.Error); ok && err.Timeout() {
			log.Printf("[server:138]]onnection to %v timed out", conn.RemoteAddr())
			if bytes_received {
				//At least some bytes arrived
				log.Printf("Got some bytes")
				res := &Response{}
				res.HandleBadRequest()
				_ = res.Write(conn)
				time.Sleep(500 * time.Millisecond)
				_ = conn.Close()

			} else {
				//No bytes, direct cutoff
				log.Printf("Got no Bytes")
				//res := &Response{}
				//res.HandleBadRequest()
				//_ = res.Write(conn)
				//time.Sleep(50 * time.Millisecond)
				_ = conn.Close()
			}

			return
		}

		if errors.Is(err, err404) {
			log.Printf("Handle 404 error: %v", err)
			res := &Response{}
			res.HandleNotFound(req)
			_ = res.Write(conn)
			_ = conn.Close()
			return
		}
		if err != nil {
			log.Printf("Handle bad request for error: %v", err)
			res := &Response{}
			res.HandleBadRequest()
			_ = res.Write(conn)
			_ = conn.Close()
			return
		}

		for each_k, each_v := range req.Header {
			log.Printf("Header[%s]: %s\r\n", each_k, each_v)
		}
		// Handle good request
		log.Printf("Handle good request: ")
		res := s.HandleGoodRequest(req)

		if res.StatusCode == statusNotFound {
			err = res.Write(conn)
			conn.Close()
		} else {
			err = res.Write(conn)
			if err != nil {
				fmt.Println(err)
			}
		}
		// Close conn if requested
		if req.Close == true {
			conn.Close()
		}
	}
}

// HandleGoodRequest handles the valid req and generates the corresponding res.
func (s *Server) HandleGoodRequest(req *Request) (res *Response) {
	res = &Response{}
	res.HandleOK(req, s.DocRoot)
	if strings.HasSuffix(req.URL, "/") {
		req.URL += "index.html"
	}

	res.Header = make(map[string]string)
	if req.Close {
		res.Header["Connection"] = "close"
	}
	res.FilePath = filepath.Join(s.DocRoot, req.URL)
	res.FilePath = filepath.Clean(res.FilePath)
	file, err := os.Stat(res.FilePath)
	if err != nil || file == nil {
		// Not Found
		res.HandleNotFound(req)
		return res
	}
	res.Header["Date"] = FormatTime(time.Now())
	// Hint: use the other methods below
	if err != nil {
		log.Fatalf(err.Error())
	}
	res.Header["Last-Modified"] = FormatTime(file.ModTime())

	res.Header["Content-Type"] = MIMETypeByExtension(filepath.Ext(res.FilePath))
	res.Header["Content-Length"] = strconv.FormatInt(file.Size(), 10)

	return res

}

// HandleOK prepares res to be a 200 OK response
// ready to be written back to client.
func (res *Response) HandleOK(req *Request, path string) {
	res.init()
	res.StatusCode = statusOK
}

// HandleBadRequest prepares res to be a 400 Bad Request response
// ready to be written back to client.
func (res *Response) HandleBadRequest() {
	res.init()
	res.StatusCode = statusMethodNotAllowed
	res.Header = make(map[string]string)
	res.Header["Connection"] = "close"
	res.Header["Date"] = ""
	res.FilePath = ""
}

// HandleNotFound prepares res to be a 404 Not Found response
// ready to be written back to client.
func (res *Response) HandleNotFound(req *Request) {
	res.init()
	res.StatusCode = statusNotFound
	res.FilePath = ""
	res.Header["Date"] = ""
}
