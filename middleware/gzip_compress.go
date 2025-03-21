// Copyright 2018 xgfone
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package middleware

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/xmx/ship"
)

// GZipConfig is used to configure the GZIP middleware.
type GZipConfig struct {
	// Level is the compression level, range [-1, 9].
	//
	// Default: -1 (default compression level)
	Level int

	// Domains is the host domains enabling the gzip compression.
	// If empty, compress all the requests to all the host domains.
	//
	// The domain name supports the exact, prefix and suffix match. For example,
	//   Exact:  www.example.com
	//   Prefix: www.example.*
	//   Suffix: *.example.com
	//
	// Default: nil
	Domains []string
}

// Gzip returns a middleware to compress the response body by GZIP.
//
// Notice:
//  1. the returned gzip middleware will always compress it,
//     no matter whether the response body is empty or not.
//  2. the gzip middleware must be the last to handle the response.
//     If returning an error stands for the failure result, therefore,
//     it should be handled before compressing the response body,
//     that's, the error handler middleware must be appended
//     after the GZip middleware.
func Gzip(config *GZipConfig) Middleware {
	var conf GZipConfig
	if config != nil {
		conf = *config
	}

	if conf.Level < gzip.HuffmanOnly || conf.Level > gzip.BestCompression {
		panic(fmt.Errorf("gzip: invalid compression level '%d'", conf.Level))
	}

	gpool := sync.Pool{New: func() interface{} {
		w, err := gzip.NewWriterLevel(nil, conf.Level)
		if err != nil {
			panic(err)
		}
		return &gzipResponse{w: w}
	}}

	releaseGzipResponse := func(r *gzipResponse) { r.w.Close(); gpool.Put(r) }
	acquireGzipResponse := func(w http.ResponseWriter) (r *gzipResponse) {
		r = gpool.Get().(*gzipResponse)
		r.ResponseWriter = w
		r.w.Reset(w)
		return
	}

	var exactDomains []string
	var prefixDomains []string
	var suffixDomains []string
	for _, domain := range conf.Domains {
		if domain == "" {
			panic("GZip: empty domain")
		} else if strings.HasPrefix(domain, "*.") {
			suffixDomains = append(suffixDomains, domain[1:])
		} else if strings.HasSuffix(domain, ".*") {
			prefixDomains = append(prefixDomains, domain[:len(domain)-1])
		} else {
			exactDomains = append(exactDomains, domain)
		}
	}

	noDomain := len(conf.Domains) == 0
	matchDomain := func(host string) bool {
		for i, _len := 0, len(exactDomains); i < _len; i++ {
			if exactDomains[i] == host {
				return true
			}
		}
		for i, _len := 0, len(prefixDomains); i < _len; i++ {
			if strings.HasPrefix(host, prefixDomains[i]) {
				return true
			}
		}
		for i, _len := 0, len(suffixDomains); i < _len; i++ {
			if strings.HasSuffix(host, suffixDomains[i]) {
				return true
			}
		}
		return false
	}

	return func(next ship.Handler) ship.Handler {
		return func(ctx *ship.Context) error {
			if strings.Contains(ctx.GetReqHeader(ship.HeaderAcceptEncoding), "gzip") {
				if noDomain || matchDomain(splitHost(ctx.Host())) {
					ctx.AddRespHeader(ship.HeaderVary, ship.HeaderAcceptEncoding)
					ctx.SetRespHeader(ship.HeaderContentEncoding, "gzip")

					resp := ctx.ResponseWriter()
					gresp := acquireGzipResponse(resp)
					defer releaseGzipResponse(gresp)
					ctx.SetResponse(gresp)
				}
			}

			return next(ctx)
		}
	}
}

type gzipResponse struct {
	http.ResponseWriter
	w *gzip.Writer
}

func (g *gzipResponse) Write(b []byte) (int, error) { return g.w.Write(b) }
func (g *gzipResponse) Flush()                      { g.w.Flush() }

func splitHost(hostport string) (host string) {
	host, _ = ship.SplitHostPort(hostport)
	return
}
