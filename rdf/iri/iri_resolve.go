package iri

// Code in this file is derived from
// https://github.com/golang/go/blob/master/src/net/url/url.go

// License of original url.go code:
//
// Copyright (c) 2009 The Go Authors. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

import "strings"

// HasAuthority returns the auth part of the IRI.
func (p *parts) HasAuthority() bool {
	return p.emptyAuth || p.host != "" || p.port != "" || p.userInfo != ""
}

// Scheme returns the scheme part of the IRI.
func (p *parts) Scheme() string {
	return p.scheme
}

// Host returns the host part of the IRI.
func (p *parts) Host() string {
	return p.host
}

// User returns the user part of the IRI.
func (p *parts) User() string {
	return p.userInfo
}

// Port returns the port part of the IRI.
func (p *parts) Port() string {
	return p.port
}

// Path returns the path part of the IRI.
func (p *parts) Path() string {
	return p.path
}

// Query returns the query part of the IRI.
func (p *parts) Query() *string {
	if p.query != "" {
		s := p.query[1:]
		return &s
	}
	return nil
}

// Fragment returns the fragment part of the IRI.
func (p *parts) Fragment() (string, bool) {
	return p.fragment, p.emptyFragment || p.fragment != ""
}

type resolveRefArg interface {
	HasAuthority() bool
	Scheme() string
	Host() string
	User() string
	Port() string
	Path() string
	Query() *string
	Fragment() (string, bool)
}

func resolveReference(base, ref resolveRefArg) *parts {
	refFrag, refHasFrag := ref.Fragment()
	url := &parts{
		scheme:        ref.Scheme(),
		emptyAuth:     ref.HasAuthority() && ref.Host() == "" && ref.Port() == "" && ref.User() == "",
		host:          ref.Host(),
		userInfo:      ref.User(),
		port:          ref.Port(),
		path:          ref.Path(),
		fragment:      refFrag,
		emptyFragment: refFrag == "" && refHasFrag,
		query:         formattedQuery(ref.Query()),
	}
	if ref.Scheme() == "" {
		url.scheme = base.Scheme()
	}
	if ref.Scheme() != "" || ref.Host() != "" || ref.User() != "" {
		// The "absoluteURI" or "net_path" cases.
		// We can ignore the error from setPath since we know we provided a
		// validly-escaped path.
		url.path = resolvePath(ref.Path(), "")
		return url
	}
	// TODO(reddaly): Deal with opaque.
	// if ref.Opaque != "" {
	// 	url.User = nil
	// 	url.host = ""
	// 	url.Path = ""
	// 	return url
	// }
	if ref.Path() == "" && ref.Query() == nil {
		url.query = formattedQuery(base.Query())

		if !refHasFrag {
			baseFrag, baseHasFrag := base.Fragment()
			url.fragment = baseFrag
			url.emptyFragment = baseFrag == "" && baseHasFrag
		}
	}
	// The "abs_path" or "rel_path" cases.
	url.host = base.Host()
	url.userInfo = base.User()
	url.path = resolvePath(base.Path(), ref.Path())
	return url
}

func formattedQuery(q *string) string {
	if q == nil {
		return ""
	}
	return "?" + *q
}

// resolvePath applies special path segments from refs and applies
// them to base, per RFC 3986.
func resolvePath(base, ref string) string {
	var full string
	if ref == "" {
		full = base
	} else if ref[0] != '/' {
		i := strings.LastIndex(base, "/")
		full = base[:i+1] + ref
	} else {
		full = ref
	}
	if full == "" {
		return ""
	}

	var (
		last string
		elem string
		i    int
		dst  strings.Builder
	)
	first := true
	remaining := full
	for i >= 0 {
		i = strings.IndexByte(remaining, '/')
		if i < 0 {
			last, elem, remaining = remaining, remaining, ""
		} else {
			elem, remaining = remaining[:i], remaining[i+1:]
		}
		if elem == "." {
			first = false
			// drop
			continue
		}

		if elem == ".." {
			str := dst.String()
			index := strings.LastIndexByte(str, '/')

			dst.Reset()
			if index == -1 {
				first = true
			} else {
				dst.WriteString(str[:index])
			}
		} else {
			if !first {
				dst.WriteByte('/')
			}
			dst.WriteString(elem)
			first = false
		}
	}

	if last == "." || last == ".." {
		dst.WriteByte('/')
	}

	return "/" + strings.TrimPrefix(dst.String(), "/")
}
