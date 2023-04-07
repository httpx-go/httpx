package httpx

import (
	"context"
	"crypto/tls"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Http is a provider via "net/http".
var Http = &httpProvider{}

// httpProvider implements httpx.Provider.
type httpProvider struct {
	contextPool    sync.Pool
	requestPool    sync.Pool
	responsePool   sync.Pool
	disableRelease bool
}

func (p *httpProvider) acquireContext() *httpContext {
	v := p.contextPool.Get()
	if v == nil {
		return &httpContext{
			req:  p.acquireRequest(),
			resp: p.acquireResponse(),
		}
	}
	return v.(*httpContext)
}

func (p *httpProvider) acquireRequest() *httpRequest {
	v := p.requestPool.Get()
	if v == nil {
		return &httpRequest{req: &http.Request{}}
	}
	return v.(*httpRequest)
}

func (p *httpProvider) acquireResponse() *httpResponse {
	v := p.responsePool.Get()
	if v == nil {
		return &httpResponse{}
	}
	return v.(*httpResponse)
}

func (p *httpProvider) AcquireContext() Context {
	return p.acquireContext()
}

func (p *httpProvider) AcquireRequest() Request {
	return p.acquireRequest()
}

func (p *httpProvider) AcquireResponse() Response {
	return p.acquireResponse()
}

func (p *httpProvider) ReleaseContext(ctx Context) {
	if p.disableRelease {
		return
	}
	ctx.Reset()
	p.contextPool.Put(ctx)
}

func (p *httpProvider) ReleaseRequest(req Request) {
	if p.disableRelease {
		return
	}
	req.Reset()
	p.requestPool.Put(req)
}

func (p *httpProvider) ReleaseResponse(resp Response) {
	if p.disableRelease {
		return
	}
	resp.Reset()
	p.responsePool.Put(resp)
}

func (p *httpProvider) SetEnableRelease(enable bool) {
	p.disableRelease = !enable
}

func (p *httpProvider) ListenAndServe(addr string, h Handler) error {
	return http.ListenAndServe(addr, http.HandlerFunc(func(hw http.ResponseWriter, hr *http.Request) {
		ctx := AcquireContext()
		defer func() { ReleaseContext(ctx) }()

		ctx.Response().SetHttpResponseWriter(hw)
		ctx.Request().SetHttpRequest(hr)
		h.Handle(ctx)
	}))
}

func (p *httpProvider) ListenAndServeTLS(addr, certFile, keyFile string, h Handler) error {
	return http.ListenAndServeTLS(addr, certFile, keyFile, http.HandlerFunc(func(hw http.ResponseWriter, hr *http.Request) {
		ctx := AcquireContext()
		defer func() { ReleaseContext(ctx) }()

		ctx.Response().SetHttpResponseWriter(hw)
		ctx.Request().SetHttpRequest(hr)
		h.Handle(ctx)
	}))
}

func (p *httpProvider) HttpHandler(h http.Handler) Handler {
	return &httpHandler{h}
}

func (p *httpProvider) HttpHandlerFunc(fn http.HandlerFunc) HandlerFunc {
	return httpHandlerFunc(fn)
}

// httpContext implements httpx.Context.
type httpContext struct {
	req  *httpRequest
	resp *httpResponse
}

var _ Context = (*httpContext)(nil)

func (c *httpContext) Request() Request {
	if c.req == nil {
		c.req = Http.acquireRequest()
	}
	return c.req
}

func (c *httpContext) Response() Response {
	if c.resp == nil {
		c.resp = Http.acquireResponse()
	}
	return c.resp
}

func (c *httpContext) Reset() {
	if c.req != nil {
		Http.ReleaseRequest(c.req)
		c.req = nil
	}
	if c.resp != nil {
		Http.ReleaseResponse(c.resp)
		c.resp = nil
	}
}

// httpValues implements httpx.Values.
type httpValues struct {
	v *url.Values
}

var _ Values = (*httpValues)(nil)

func (v *httpValues) Each(fn func(name string, values []string)) {
	for name, values := range *v.v {
		fn(name, values)
	}
}

func (v *httpValues) Set(name string, values ...string) {
	(*v.v)[name] = values
}

func (v *httpValues) Add(name, value string) {
	v.v.Add(name, value)
}

func (v *httpValues) Del(name string) {
	v.v.Del(name)
}

func (v *httpValues) Has(name string) bool {
	return v.v.Has(name)
}

func (v *httpValues) Value(name string) string {
	return v.v.Get(name)
}

func (v *httpValues) Values(name string) []string {
	return (*v.v)[name]
}

func (v *httpValues) Len() int {
	return len(*v.v)
}

func (v *httpValues) Reset() {
	for k := range *v.v {
		delete(*v.v, k)
	}
}

// httpHeader implements httpx.Header.
type httpHeader struct {
	http.Header
}

var _ Header = (*httpHeader)(nil)

func (h *httpHeader) Each(fn func(name string, values []string)) {
	for name, values := range h.Header {
		fn(name, values)
	}
}

func (h *httpHeader) Set(name string, values ...string) {
	h.Header[name] = values
}

func (h *httpHeader) Delete(name string) {
	delete(h.Header, name)
}

func (h *httpHeader) Has(name string) bool {
	_, ok := (h.Header)[name]
	return ok
}

func (h *httpHeader) Value(name string) string {
	if vs := h.Header.Values(name); len(vs) > 0 {
		return vs[0]
	}
	return ""
}

func (h *httpHeader) WriteTo(w io.Writer) error {
	return h.Header.Write(w)
}

func (h *httpHeader) Len() int {
	return len(h.Header)
}

func (h *httpHeader) Reset() {
	for k := range h.Header {
		delete(h.Header, k)
	}
}

// httpRequest implements httpx.Request.
type httpRequest struct {
	req      *http.Request
	header   *httpHeader
	form     *httpValues
	postForm *httpValues
	reqOrg   *http.Request
}

var _ Request = (*httpRequest)(nil)

func (r *httpRequest) Header() Header {
	if r.header == nil {
		r.header = &httpHeader{}
	}
	if r.req.Header == nil {
		r.req.Header = http.Header{}
	}
	r.header.Header = r.req.Header
	return r.header
}

func (r *httpRequest) Trailer() Header {
	return &httpHeader{Header: r.req.Trailer}
}

func (r *httpRequest) Method() string {
	return r.req.Method
}

func (r *httpRequest) URL() *url.URL {
	return r.req.URL
}

func (r *httpRequest) Proto() string {
	return r.req.Proto
}

func (r *httpRequest) ProtoMajor() int {
	return r.req.ProtoMajor
}

func (r *httpRequest) ProtoMinor() int {
	return r.req.ProtoMinor
}

func (r *httpRequest) Host() string {
	return r.req.Host
}

func (r *httpRequest) RemoteAddr() string {
	return r.req.RemoteAddr
}

func (r *httpRequest) RequestURI() string {
	return r.req.RequestURI
}

func (r *httpRequest) ContentLength() int64 {
	return r.req.ContentLength
}

func (r *httpRequest) Body() io.ReadCloser {
	return r.req.Body
}

func (r *httpRequest) TLS() *tls.ConnectionState {
	return r.req.TLS
}

func (r *httpRequest) Form() Values {
	if r.form == nil {
		r.form = &httpValues{}
	}
	r.form.v = &r.req.Form
	return r.form
}

func (r *httpRequest) PostForm() Values {
	if r.postForm == nil {
		r.postForm = &httpValues{}
	}
	r.postForm.v = &r.req.PostForm
	return r.postForm
}

func (r *httpRequest) MultipartmForm() *multipart.Form {
	return r.req.MultipartForm
}

func (r *httpRequest) SetMethod(method string) {
	r.req.Method = method
}

func (r *httpRequest) SetURL(u *url.URL) {
	r.req.URL = u
}

func (r *httpRequest) SetProto(proto string) {
	r.req.Proto = proto
}

func (r *httpRequest) SetHost(host string) {
	r.req.Host = host
}

func (r *httpRequest) SetRemoteAddr(addr string) {
	r.req.RemoteAddr = addr
}

func (r *httpRequest) SetRequestURI(requestURI string) {
	r.req.RequestURI = requestURI
}

func (r *httpRequest) SetContentLength(contentLength int64) {
	r.req.ContentLength = contentLength
}

func (r *httpRequest) SetBody(body io.ReadCloser) {
	r.req.Body = body
}

func (r *httpRequest) SetTLS(connectionState *tls.ConnectionState) {
	r.req.TLS = connectionState
}

func (r *httpRequest) Context() context.Context {
	return r.req.Context()
}

func (r *httpRequest) WithContext(ctx context.Context) Request {
	r.SetHttpRequest(r.req.WithContext(ctx))
	return r
}

func (r *httpRequest) SetHttpRequest(hr *http.Request) {
	r.Reset()
	r.reqOrg = r.req
	r.req = hr
}

func (r *httpRequest) Reset() {
	if r.reqOrg != nil {
		r.req = r.reqOrg
		r.reqOrg = nil
	}
	if r.header != nil {
		r.header.Reset()
	}
	if r.form != nil {
		r.form.Reset()
	}
	if r.postForm != nil {
		r.postForm.Reset()
	}
	r.req.Method = ""
	r.req.URL = nil
	r.req.Proto = ""
	r.req.ProtoMajor = 0
	r.req.ProtoMinor = 0
	r.req.Host = ""
	r.req.RemoteAddr = ""
	r.req.RequestURI = ""
	r.req.ContentLength = 0
	r.req.Body = nil
	r.req.TLS = nil
}

// httpResponse implements httpx.Response.
type httpResponse struct {
	w      http.ResponseWriter
	header *httpHeader
}

var _ Response = (*httpResponse)(nil)

func (r *httpResponse) Header() Header {
	if r.header == nil {
		r.header = &httpHeader{}
	}
	if r.w != nil {
		r.header.Header = r.w.Header()
	}
	if r.header.Header == nil {
		r.header.Header = http.Header{}
	}
	return r.header
}

func (r *httpResponse) Write(b []byte) (int, error) {
	if r.w == nil {
		return 0, nil
	}
	return r.w.Write(b)
}

func (r *httpResponse) WriteHeader(statusCode int) {
	if r.w == nil {
		return
	}
	r.w.WriteHeader(statusCode)
}

func (r *httpResponse) SetHttpResponseWriter(hw http.ResponseWriter) {
	r.w = hw
}

func (r *httpResponse) Reset() {
	if r.header != nil {
		r.header.Reset()
	}
	r.w = nil
}

// httpHandler implements httpx.Handler.
type httpHandler struct {
	http.Handler
}

func (h *httpHandler) Handle(ctx Context) {
	hctx, ok := ctx.(*httpContext)
	if ok {
		h.ServeHTTP(hctx.resp.w, hctx.req.req)
		return
	}
	w := ToHttpResponseWriter(ctx.Response())
	r := ToHttpRequest(ctx.Request())
	h.ServeHTTP(w, r)
}

func httpHandlerFunc(fn http.HandlerFunc) HandlerFunc {
	return HandlerFunc(func(ctx Context) {
		hctx, ok := ctx.(*httpContext)
		if ok {
			fn(hctx.resp.w, hctx.req.req)
			return
		}
		w := ToHttpResponseWriter(ctx.Response())
		r := ToHttpRequest(ctx.Request())
		fn(w, r)
	})
}

// httpResponseWriter implements http.ResponseWriter for ToHttpResponseWriter.
type httpResponseWriter struct {
	resp Response
}

func (w *httpResponseWriter) Header() http.Header {
	return ToHttpHeader(w.resp.Header())
}

func (w *httpResponseWriter) Write(b []byte) (int, error) {
	return w.resp.Write(b)
}

func (w *httpResponseWriter) WriteHeader(statusCode int) {
	w.resp.WriteHeader(statusCode)
}

// ToHttpResponseWriter converts httpx.Response to http.ResponseWriter.
func ToHttpResponseWriter(w Response) http.ResponseWriter {
	return &httpResponseWriter{w}
}

// ToHttpHeader converts httpx.Header to http.Header.
func ToHttpHeader(h Header) http.Header {
	if hh, ok := h.(*httpHeader); ok {
		return hh.Header
	}
	hh := http.Header{}
	h.Each(func(name string, values []string) {
		hh[name] = values
	})
	return hh
}

// ToHttpForm converts httpx.Values to url.Values.
func ToHttpForm(f Values) url.Values {
	if hf, ok := f.(*httpValues); ok {
		return *hf.v
	}
	hf := url.Values{}
	f.Each(func(name string, values []string) {
		hf[name] = values
	})
	return hf
}

// ToHttpRequest converts httpx.Request to *http.Request.
func ToHttpRequest(r Request) *http.Request {
	if hr, ok := r.(*httpRequest); ok {
		return hr.req
	}
	var transferEncodings []string
	for _, v := range r.Header().Values("TransferEncoding") {
		vs := strings.Split(v, ",")
		for _, vss := range vs {
			transferEncodings = append(transferEncodings, strings.TrimSpace(vss))
		}
	}
	return &http.Request{
		Method:           r.Method(),
		URL:              r.URL(),
		Proto:            r.Proto(),
		ProtoMajor:       r.ProtoMajor(),
		ProtoMinor:       r.ProtoMinor(),
		Header:           ToHttpHeader(r.Header()),
		Body:             r.Body(),
		ContentLength:    r.ContentLength(),
		TransferEncoding: transferEncodings,
		Close:            false, // TODO
		Host:             r.Host(),
		Form:             ToHttpForm(r.Form()),
		PostForm:         ToHttpForm(r.PostForm()),
		MultipartForm:    r.MultipartmForm(),
		Trailer:          ToHttpHeader(r.Trailer()),
		RemoteAddr:       r.RemoteAddr(),
		RequestURI:       r.RequestURI(),
		TLS:              r.TLS(),
	}
}

func init() {
	RegisterProvider(Http)
}
