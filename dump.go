package main

import (
	"bytes"
	"fmt"
	"github.com/urfave/cli"
	"golang.org/x/text/encoding/htmlindex"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
)

func Dump(c *cli.Context) {
	http.HandleFunc("/dump", func(hw http.ResponseWriter, r *http.Request) {
		charset := ""
		w := os.Stderr
		ct := r.Header.Get("Content-Type")
		if ct != "" {
			_, params, err := mime.ParseMediaType(ct)
			if err == nil {
				charset = params["charset"]
			}
		}
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			hw.WriteHeader(500)
			return
		}
		if charset != "" {
			encoding, err := htmlindex.Get(charset)
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, "unknown encoding")
				hw.WriteHeader(500)
				return
			}
			decoded, err := encoding.NewDecoder().Bytes(body)
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, "error decoding body")
				hw.WriteHeader(500)
				return
			}
			body = decoded
		}
		_, _ = io.WriteString(w, "HEADERS\n")
		for k := range r.Header {
			_, _ = io.WriteString(w, k + " = " + r.Header.Get(k) + "\n")
		}
		_, _ = io.WriteString(w, "\n")
		_, _ = io.WriteString(w, "BODY\n")
		_, _ = w.Write(body)
		_, _ = io.WriteString(w, "\n\n")
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		err = r.ParseMultipartForm(10000000)
		_, _ = io.WriteString(w, "FORMS\n")
		if err != nil {
			_, _ = io.WriteString(w, err.Error())
			_, _ = io.WriteString(w, "\n")
		}
		for k := range r.Form {
			_, _ = io.WriteString(w, k + " = " + r.Form.Get(k) + "\n")
		}
	})
	_ = http.ListenAndServe("127.0.0.1:8081", nil)
}
