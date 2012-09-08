package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

var (
	v4dir = "aws4_testsuite"
)

type v4TestFiles struct {
	base string
	req  []byte
	sreq []byte

	request *http.Request
	body    io.ReadSeeker
}

func readTestFiles(t *testing.T) chan *v4TestFiles {
	d, err := os.Open(v4dir)
	if err != nil {
		t.Fatal(err)
	}
	f, err := d.Readdirnames(0)
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(f)

	files := make([]string, 0)
	for i := 0; i < len(f)-4; {
		if filepath.Ext(f[i]) == ".authz" &&
			filepath.Ext(f[i+1]) == ".creq" &&
			filepath.Ext(f[i+2]) == ".req" &&
			filepath.Ext(f[i+3]) == ".sreq" &&
			filepath.Ext(f[i+4]) == ".sts" {
			files = append(files, f[i][:len(f[i])-6])
			i += 5
		} else {
			i++
		}
	}

	ch := make(chan *v4TestFiles)
	go func() {
		for _, f := range files {
			var err error
			d := new(v4TestFiles)
			d.base = f

			// read in the raw request and convert it to go's internal format
			d.req, err = ioutil.ReadFile(v4dir + "/" + f + ".req")
			if err != nil {
				t.Error("reading", d.base, err)
				continue
			}
			// go doesn't like post requests with spaces in them
			if d.base == "post-vanilla-query-nonunreserved" ||
				d.base == "post-vanilla-query-space" ||
				d.base == "get-slashes" {
				// skip tests with spacing in URLs or invalid escapes or
				// triling slashes
				continue
			} else {
				// go doesn't like lowercase http
				fixed := bytes.Replace(d.req, []byte("http"), []byte("HTTP"), 1)
				reader := bufio.NewReader(bytes.NewBuffer(fixed))
				d.request, err = http.ReadRequest(reader)
				if err != nil {
					t.Error("parsing", d.base, "request", err)
					continue
				}
				delete(d.request.Header, "User-Agent")
				if i := bytes.Index(d.req, []byte("\n\n")); i != -1 {
					d.body = bytes.NewReader(d.req[i+2:])
					d.request.Body = ioutil.NopCloser(d.body)
				}
			}

			d.sreq, err = ioutil.ReadFile(v4dir + "/" + f + ".sreq")
			if err != nil {
				t.Error("reading", d.base, err)
				continue
			}

			ch <- d
		}
		close(ch)
	}()
	return ch
}

func TestSignatureVersion4(t *testing.T) {
	date := time.Date(2011, time.September, 9, 0, 0, 0, 0, time.UTC)
	signature := NewSignature("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"20110909/us-east-1/host/aws4_request", date, USEast, "host")

	tests := readTestFiles(t)
	for f := range tests {
		err := signature.Sign(f.request)
		if err != nil {
			t.Error(err)
			continue
		}

		var sreqBuffer bytes.Buffer
		i := bytes.Index(f.req, []byte("\n\n"))
		_, err = sreqBuffer.Write(f.req[:i+1])
		if err != nil {
			t.Error(err)
			continue
		}
		// _, err = sreqBuffer.WriteString(fmt.Sprintf("Authorization: %s\n\n",
		// 	authz))
		_, err = sreqBuffer.WriteString(fmt.Sprintf("Authorization: %s\n\n",
			f.request.Header.Get("Authorization")))
		if err != nil {
			t.Error(err)
			continue
		}
		f.body.Seek(0, 0)
		_, err = io.Copy(&sreqBuffer, f.request.Body)
		if err != nil {
			t.Error(err)
			continue
		}
		sreq := sreqBuffer.Bytes()
		if !bytes.Equal(sreq, f.sreq) {
			t.Error(f.base, "signed request")
			t.Logf("got:\n%s", sreq)
			t.Logf("want:\n%s", f.sreq)
		}
	}
}

func BenchmarkNewSignature(b *testing.B) {
	t := time.Now()
	for i := 0; i < b.N; i++ {
		_ = NewSignature("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
			"20110909/us-east-1/host/aws4_request", t, USEast, "service")
	}
}

func BenchmarkSignatureSign(b *testing.B) {
	b.StopTimer()
	date := time.Date(2011, time.September, 9, 0, 0, 0, 0, time.UTC)
	signature := NewSignature("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"20110909/us-east-1/host/aws4_request", date, USEast, "service")
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		rawRequest := []byte(`POST / HTTP/1.1
Content-Type:application/x-www-form-urlencoded
Date:Mon, 09 Sep 2011 23:36:00 GMT
Host:host.foo.com

foo=bar`)
		reader := bufio.NewReader(bytes.NewBuffer(rawRequest))
		request, err := http.ReadRequest(reader)
		if err != nil {
			b.Fatal(err)
		}
		delete(request.Header, "User-Agent")
		if i := bytes.Index(rawRequest, []byte("\n\n")); i != -1 {
			body := bytes.NewReader(rawRequest[i+2:])
			request.Body = ioutil.NopCloser(body)
		}
		b.StartTimer()
		_ = signature.Sign(request)
	}
}
