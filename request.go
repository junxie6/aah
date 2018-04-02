// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/ahttp source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package ahttp

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"aahframework.org/essentials.v0"
)

const (
	jsonpReqParamKey     = "callback"
	ajaxHeaderValue      = "XMLHttpRequest"
	websocketHeaderValue = "websocket"
)

var requestPool = &sync.Pool{New: func() interface{} { return &Request{} }}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Package methods
//___________________________________

// ParseRequest method populates the given aah framework `ahttp.Request`
// instance from Go HTTP request.
func ParseRequest(r *http.Request, req *Request) *Request {
	req.Scheme = identifyScheme(r)
	req.Host = host(r)
	req.Proto = r.Proto
	req.Method = r.Method
	req.Path = r.URL.Path
	req.Header = r.Header
	req.Params = &Params{Query: r.URL.Query()}
	req.Referer = getReferer(r.Header)
	req.UserAgent = r.Header.Get(HeaderUserAgent)
	req.ClientIP = clientIP(r)
	req.IsGzipAccepted = strings.Contains(r.Header.Get(HeaderAcceptEncoding), "gzip")
	req.Raw = r

	return req
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Request
//___________________________________

// Request type extends `http.Request` and provides multiple helper methods
// per industry RFC guideline for aah framework.
type Request struct {
	// Scheme value is protocol; it's a derived value in the order as below.
	//  - `X-Forwarded-Proto` is not empty return value as is
	//  - `http.Request.TLS` is not nil value is `https`
	//  - `http.Request.TLS` is nil value is `http`
	Scheme string

	// Host value of the HTTP 'Host' header (e.g. 'example.com:8080').
	Host string

	// Proto value of the current HTTP request protocol. (e.g. HTTP/1.1, HTTP/2.0)
	Proto string

	// Method request method e.g. `GET`, `POST`, etc.
	Method string

	// Path the request URL Path e.g. `/app/login.html`.
	Path string

	// Header request HTTP headers
	Header http.Header

	// Params contains values from Path, Query, Form and File.
	Params *Params

	// Referer value of the HTTP 'Referrer' (or 'Referer') header.
	Referer string

	// UserAgent value of the HTTP 'User-Agent' header.
	UserAgent string

	// ClientIP remote client IP address aka Remote IP. Parsed in the order of
	// `X-Forwarded-For`, `X-Real-IP` and finally `http.Request.RemoteAddr`.
	ClientIP string

	// IsGzipAccepted is true if the HTTP client accepts Gzip response,
	// otherwise false.
	IsGzipAccepted bool

	// Raw an object of Go HTTP server, direct interaction with
	// raw object is not encouraged.
	//
	// DEPRECATED: Raw field to be unexported on v1 release, use `Req.Unwarp()` instead.
	Raw *http.Request

	locale            *Locale
	contentType       *ContentType
	acceptContentType *ContentType
	acceptEncoding    *AcceptSpec
}

// AcceptContentType method returns negotiated value.
//
// The resolve order is-
//
// 1) URL extension
//
// 2) Accept header (As per RFC7231 and vendor type as per RFC4288)
//
// Most quailfied one based on quality factor otherwise default is Plain text.
func (r *Request) AcceptContentType() *ContentType {
	if r.acceptContentType == nil {
		r.acceptContentType = NegotiateContentType(r.Unwrap())
	}
	return r.acceptContentType
}

// SetAcceptContentType method is used to set Accept ContentType instance.
func (r *Request) SetAcceptContentType(contentType *ContentType) *Request {
	r.acceptContentType = contentType
	return r
}

// AcceptEncoding method returns negotiated value from HTTP Header the `Accept-Encoding`
// As per RFC7231.
//
// Most quailfied one based on quality factor.
func (r *Request) AcceptEncoding() *AcceptSpec {
	if r.acceptEncoding == nil {
		if specs := ParseAcceptEncoding(r.Unwrap()); specs != nil {
			r.acceptEncoding = specs.MostQualified()
		}
	}
	return r.acceptEncoding
}

// SetAcceptEncoding method is used to accept encoding spec instance.
func (r *Request) SetAcceptEncoding(encoding *AcceptSpec) *Request {
	r.acceptEncoding = encoding
	return r
}

// Cookie method returns a named cookie from HTTP request otherwise error.
func (r *Request) Cookie(name string) (*http.Cookie, error) {
	return r.Unwrap().Cookie(name)
}

// Cookies method returns all the cookies from HTTP request.
func (r *Request) Cookies() []*http.Cookie {
	return r.Unwrap().Cookies()
}

// ContentType method returns the parsed value of HTTP header `Content-Type` per RFC1521.
func (r *Request) ContentType() *ContentType {
	if r.contentType == nil {
		r.contentType = ParseContentType(r.Unwrap())
	}
	return r.contentType
}

// SetContentType method is used to set ContentType instance.
func (r *Request) SetContentType(contType *ContentType) *Request {
	r.contentType = contType
	return r
}

// Locale method returns negotiated value from HTTP Header `Accept-Language`
// per RFC7231.
func (r *Request) Locale() *Locale {
	if r.locale == nil {
		r.locale = NegotiateLocale(r.Unwrap())
	}
	return r.locale
}

// SetLocale method is used to set locale instance in to aah request.
func (r *Request) SetLocale(locale *Locale) *Request {
	r.locale = locale
	return r
}

// IsJSONP method returns true if request URL query string has "callback=function_name".
// otherwise false.
func (r *Request) IsJSONP() bool {
	return !ess.IsStrEmpty(r.QueryValue(jsonpReqParamKey))
}

// IsAJAX method returns true if request header `X-Requested-With` is
// `XMLHttpRequest` otherwise false.
func (r *Request) IsAJAX() bool {
	return r.Header.Get(HeaderXRequestedWith) == ajaxHeaderValue
}

// IsWebSocket method returns true if request is WebSocket otherwise false.
func (r *Request) IsWebSocket() bool {
	return r.Header.Get(HeaderUpgrade) == websocketHeaderValue
}

// URL method return underlying request URL instance.
func (r *Request) URL() *url.URL {
	return r.Unwrap().URL
}

// PathValue method returns value for given Path param key otherwise empty string.
// For eg.: /users/:userId => PathValue("userId")
func (r *Request) PathValue(key string) string {
	return r.Params.PathValue(key)
}

// QueryValue method returns value for given URL query param key
// otherwise empty string.
func (r *Request) QueryValue(key string) string {
	return r.Params.QueryValue(key)
}

// QueryArrayValue method returns array value for given URL query param key
// otherwise empty string slice.
func (r *Request) QueryArrayValue(key string) []string {
	return r.Params.QueryArrayValue(key)
}

// FormValue method returns value for given form key otherwise empty string.
func (r *Request) FormValue(key string) string {
	return r.Params.FormValue(key)
}

// FormArrayValue method returns array value for given form key
// otherwise empty string slice.
func (r *Request) FormArrayValue(key string) []string {
	return r.Params.FormArrayValue(key)
}

// FormFile method returns the first file for the provided form key otherwise
// returns error. It is caller responsibility to close the file.
func (r *Request) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	return r.Params.FormFile(key)
}

// Body method returns the HTTP request body.
func (r *Request) Body() io.ReadCloser {
	return r.Unwrap().Body
}

// Unwrap method returns the underlying *http.Request instance of Go HTTP server,
// direct interaction with raw object is not encouraged. Use it appropriately.
func (r *Request) Unwrap() *http.Request {
	return r.Raw
}

// SaveFile method saves an uploaded multipart file for given key from the HTTP
// request into given destination
func (r *Request) SaveFile(key, dstFile string) (int64, error) {
	if ess.IsStrEmpty(dstFile) || ess.IsStrEmpty(key) {
		return 0, errors.New("ahttp: key or dstFile is empty")
	}

	if ess.IsDir(dstFile) {
		return 0, errors.New("ahttp: dstFile should not be a directory")
	}

	uploadedFile, _, err := r.FormFile(key)
	if err != nil {
		return 0, err
	}
	defer ess.CloseQuietly(uploadedFile)

	return saveFile(uploadedFile, dstFile)
}

// SaveFiles method saves an uploaded multipart file(s) for the given key
// from the HTTP request into given destination directory. It uses the filename
// as uploaded filename from the request
func (r *Request) SaveFiles(key, dstPath string) ([]int64, []error) {
	if !ess.IsDir(dstPath) {
		return []int64{0}, []error{fmt.Errorf("ahttp: destination path, '%s' is not a directory", dstPath)}
	}

	if ess.IsStrEmpty(key) {
		return []int64{0}, []error{fmt.Errorf("ahttp: form file key, '%s' is empty", key)}
	}

	var errs []error
	var sizes []int64
	for _, file := range r.Params.File[key] {
		uploadedFile, err := file.Open()
		if err != nil {
			sizes = append(sizes, 0)
			errs = append(errs, err)
			continue
		}

		if size, err := saveFile(uploadedFile, filepath.Join(dstPath, file.Filename)); err != nil {
			sizes = append(sizes, size)
			errs = append(errs, err)
		}
		ess.CloseQuietly(uploadedFile)
	}
	return sizes, errs
}

// Reset method resets request instance for reuse.
func (r *Request) Reset() {
	r.Scheme = ""
	r.Host = ""
	r.Proto = ""
	r.Method = ""
	r.Path = ""
	r.Header = nil
	r.Params = nil
	r.Referer = ""
	r.UserAgent = ""
	r.ClientIP = ""
	r.IsGzipAccepted = false
	r.Raw = nil

	r.locale = nil
	r.contentType = nil
	r.acceptContentType = nil
	r.acceptEncoding = nil
}

func (r *Request) cleanupMutlipart() {
	if r.Unwrap().MultipartForm != nil {
		r.Unwrap().MultipartForm.RemoveAll()
	}
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Params methods
//___________________________________

// Params structure holds value of Path, Query, Form and File.
type Params struct {
	Path  map[string]string
	Query url.Values
	Form  url.Values
	File  map[string][]*multipart.FileHeader
}

// PathValue method returns value for given Path param key otherwise empty string.
// For eg.: `/users/:userId` => `PathValue("userId")`.
func (p *Params) PathValue(key string) string {
	if p.Path != nil {
		if value, found := p.Path[key]; found {
			return value
		}
	}
	return ""
}

// QueryValue method returns value for given URL query param key
// otherwise empty string.
func (p *Params) QueryValue(key string) string {
	return p.Query.Get(key)
}

// QueryArrayValue method returns array value for given URL query param key
// otherwise empty string slice.
func (p *Params) QueryArrayValue(key string) []string {
	if values, found := p.Query[key]; found {
		return values
	}
	return []string{}
}

// FormValue method returns value for given form key otherwise empty string.
func (p *Params) FormValue(key string) string {
	if p.Form != nil {
		return p.Form.Get(key)
	}
	return ""
}

// FormArrayValue method returns array value for given form key
// otherwise empty string slice.
func (p *Params) FormArrayValue(key string) []string {
	if p.Form != nil {
		if values, found := p.Form[key]; found {
			return values
		}
	}
	return []string{}
}

// FormFile method returns the first file for the provided form key
// otherwise returns error. It is caller responsibility to close the file.
func (p *Params) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	if p.File != nil {
		if fh := p.File[key]; len(fh) > 0 {
			f, err := fh[0].Open()
			return f, fh[0], err
		}
		return nil, nil, fmt.Errorf("ahttp: no such key/file: %s", key)
	}
	return nil, nil, nil
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

// identifyScheme method is to identify value of protocol value. It's is derived
// one, Go language doesn't provide directly.
//  - `X-Forwarded-Proto` is not empty return value as is
//  - `http.Request.TLS` is not nil value is `https`
//  - `http.Request.TLS` is nil value is `http`
func identifyScheme(r *http.Request) string {
	scheme := r.Header.Get(HeaderXForwardedProto)
	if scheme == "" {
		if r.TLS == nil {
			return SchemeHTTP // "http"
		}
		return SchemeHTTPS // "https"
	}
	return scheme
}

// clientIP returns IP address from HTTP request, typically known as Client IP or
// Remote IP. It parses the IP in the order of X-Forwarded-For, X-Real-IP
// and finally `http.Request.RemoteAddr`.
func clientIP(req *http.Request) string {
	// Header X-Forwarded-For
	if fwdFor := req.Header.Get(HeaderXForwardedFor); !ess.IsStrEmpty(fwdFor) {
		index := strings.Index(fwdFor, ",")
		if index == -1 {
			return strings.TrimSpace(fwdFor)
		}
		return strings.TrimSpace(fwdFor[:index])
	}

	// Header X-Real-Ip
	if realIP := req.Header.Get(HeaderXRealIP); !ess.IsStrEmpty(realIP) {
		return strings.TrimSpace(realIP)
	}

	// Remote Address
	if remoteAddr, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		return strings.TrimSpace(remoteAddr)
	}

	return ""
}

func host(r *http.Request) string {
	if r.URL.Host == "" {
		return r.Host
	}
	return r.URL.Host
}

func getReferer(hdr http.Header) string {
	referer := hdr.Get(HeaderReferer)
	if referer == "" {
		return hdr.Get("Referrer")
	}
	return referer
}

func saveFile(r io.Reader, destFile string) (int64, error) {
	f, err := os.OpenFile(destFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return 0, fmt.Errorf("ahttp: %s", err)
	}
	defer ess.CloseQuietly(f)

	return io.Copy(f, r)
}
