package httpx

import (
	"context"
	"crypto/tls"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
)

// Provider is the interface for providing objects of httpx.
type Provider interface {
	// AcquireContext returns an empty Context instance from the pool.
	// The returned Context instance should be passed to ReleaseContext to
	// be returned when no longer needed.
	AcquireContext() Context

	// AcquireRequest returns an empty Request instance from the pool.
	// The returned Request instance should be passed to ReleaseRequest to
	// be returned when no longer needed.
	AcquireRequest() Request

	// AcquireResponse returns an empty Response instance from the pool.
	// The returned Response instance should be passed to ReleaseResponse to
	// be returned when no longer needed.
	AcquireResponse() Response

	// ReleaseContext return a context acquired via AcquireContext to the pool.
	// It is forbidden accessing instance and/or its' members after returning
	// it to the pool.
	ReleaseContext(ctx Context)

	// ReleaseRequest return a request acquired via AcquireRequest to the pool.
	// It is forbidden accessing instance and/or its' members after returning
	// it to the pool.
	ReleaseRequest(req Request)

	// ReleaseResponse return a response acquired via AcquireResponse to the pool.
	// It is forbidden accessing instance and/or its' members after returning
	// it to the pool.
	ReleaseResponse(res Response)

	// SetEnableRelease sets enable ReleaseContext, ReleaseRequest, and
	// ReleaseResponse. The default is enable.
	// Set disable is not normally used, but is used when assertions are required
	// for unit tests.
	SetEnableRelease(enable bool)

	// ListenAndServe listens on the TCP network address addr and then calls
	// Serve with handler to handle requests on incoming connections.
	ListenAndServe(addr string, h Handler) error

	// ListenAndServeTLS acts identically to ListenAndServe, except that it
	// expects HTTPS connections. Additionally, files containing a certificate and
	// matching private key for the server must be provided. If the certificate
	// is signed by a certificate authority, the certFile should be the concatenation
	// of the server's certificate, any intermediates, and the CA's certificate.
	ListenAndServeTLS(addr, certFile, keyFile string, h Handler) error

	// HttpHandler converts http.Handler to httpx.Handler.
	HttpHandler(h http.Handler) Handler

	// HttpHandlerFunc converts http.HandlerFunc to httpx.Handler.
	HttpHandlerFunc(fn http.HandlerFunc) HandlerFunc
}

// Context holds current HTTP request and response objects.
type Context interface {
	// Request returns httpx.Request.
	Request() Request
	// Response returns httpx.Response.
	Response() Response
	// Reset resets request and response.
	Reset()
}

// Value represents HTTP values like map[string][]string.
type Values interface {
	// Each calls fn for each names with values.
	Each(fn func(name string, values []string))
	// Set sets the named values.
	Set(name string, values ...string)
	// Add adds a value to the existing named values.
	Add(name, value string)
	// Del deletes named values.
	Del(name string)
	// Has checks the name is exists.
	Has(name string) bool
	// Value returns a first value.
	Value(name string) string
	// Values returns values.
	Values(name string) []string
	// Len returns the names count.
	Len() int
	// Reset resets all values.
	Reset()
}

// Header represents a HTTP header.
type Header interface {
	Values
	// WriteTo writes a header in wire format.
	WriteTo(w io.Writer) error
}

// Request represents a HTTP request.
type Request interface {
	// Header returns the HTTP header.
	Header() Header
	// Trailer returns the trailer HTTP header.
	Trailer() Header
	// Method returns the HTTP method.
	Method() string
	// URL returns the URI being requested.
	URL() *url.URL
	// Proto returns the protocol version for incoming server requests.
	Proto() string
	// ProtoMajor returns a major version number in the protocol version.
	ProtoMajor() int
	// ProtoMinor returns a minor version number in the protocol version.
	ProtoMinor() int
	// Host returns the host on which the URL is sought.
	Host() string
	// RemoteAddr returns the network address that sent the request.
	RemoteAddr() string
	// RequestURI returns the request target.
	RequestURI() string
	// ContentLength returns the length of the associated content.
	ContentLength() int64
	// Body returns is the request's body.
	Body() io.ReadCloser
	// TLS returns the TLS connection on which the request was received.
	TLS() *tls.ConnectionState
	// Form returns the parsed form data.
	Form() Values
	// PostForm returns the parsed form data from PATCH, POST or PUT body parameters.
	PostForm() Values
	// MultipartmForm returns the parsed multipart form, including file uploads.
	MultipartmForm() *multipart.Form
	// SetMethod sets the HTTP method.
	SetMethod(method string)
	// SetURL sets the URL.
	SetURL(u *url.URL)
	// SetProto sets the protocol version.
	SetProto(proto string)
	// SetHost sets the host.
	SetHost(host string)
	// SetRemoteAddr sets the remote addr.
	SetRemoteAddr(addr string)
	// SetRequestURI sets the request URI.
	SetRequestURI(requestURI string)
	// SetContentLength set the content length.
	SetContentLength(contentLength int64)
	// SetBody sets the request body.
	SetBody(body io.ReadCloser)
	// SetTLS sets the connection state.
	SetTLS(connectionState *tls.ConnectionState)
	// Context returns the request context.
	Context() context.Context
	// WithContext returns a shallow copy of the request with its context changed to ctx.
	WithContext(ctx context.Context) Request
	// SetHttpRequest copies headers, body, etc. to this httpx.Request.
	// Depending on the implementation, it may be re-set as an internal request.
	// Update operations on this httpx.Request may affect specified request.
	SetHttpRequest(hr *http.Request)
	// Reset resets this request.
	Reset()
}

// Response represents a HTTP response.
type Response interface {
	// Header returns a header.
	Header() Header
	// Write writes p to the HTTP response body.
	Write(p []byte) (int, error)
	// Write writes statusCode to the HTTP response header.
	WriteHeader(statusCode int)
	// SetHttpResponseWriter sets http.ResponseWriter.
	SetHttpResponseWriter(hw http.ResponseWriter)
	// Reset resets this response.
	Reset()
}

// A Handler responds to a httpx.Context.
type Handler interface {
	// Handle called by the HTTP server.
	Handle(ctx Context)
}

// The HandlerFunc type is an adapter to allow the use of ordinary functions
// as httpx.Handler. If fn is a function with the appropriate signature,
// HandlerFunc(fn) is a Handler that calls fn.
type HandlerFunc func(ctx Context)

// Handle calls fn(ctx).
func (fn HandlerFunc) Handle(ctx Context) {
	fn(ctx)
}

var defaultProvider Provider

// RegisterProvider registers a provider as default provider.
func RegisterProvider(p Provider) {
	if p == nil {
		panic("provider is nil")
	}
	defaultProvider = p
}

// AcquireContext calls AcquireContext of the registered default provider.
func AcquireContext() Context {
	return defaultProvider.AcquireContext()
}

// AcquireRequest calls AcquireRequest of the registered default provider.
func AcquireRequest() Request {
	return defaultProvider.AcquireRequest()
}

// AcquireResponse calls AcquireResponse of the registered default provider.
func AcquireResponse() Response {
	return defaultProvider.AcquireResponse()
}

// ReleaseContext calls ReleaseContext of the registered default provider.
func ReleaseContext(ctx Context) {
	defaultProvider.ReleaseContext(ctx)
}

// ReleaseRequest calls ReleaseRequest of the registered default provider.
func ReleaseRequest(req Request) {
	defaultProvider.ReleaseRequest(req)
}

// ReleaseResponse calls ReleaseResponse of the registered default provider.
func ReleaseResponse(resp Response) {
	defaultProvider.ReleaseResponse(resp)
}

// SetEnableRelease calls SetEnableRelease of the registered default provider.
func SetEnableRelease(enable bool) {
	defaultProvider.SetEnableRelease(enable)
}

// ListenAndServe calls ListenAndServe of the registered default provider.
func ListenAndServe(addr string, h Handler) error {
	return defaultProvider.ListenAndServe(addr, h)
}

// HttpHandler calls HttpHandler of the registered default provider.
func HttpHandler(handler http.Handler) Handler {
	return defaultProvider.HttpHandler(handler)
}

// HttpHandlerFunc calls HttpHandlerFunc of the registered default provider.
func HttpHandlerFunc(fn http.HandlerFunc) HandlerFunc {
	return defaultProvider.HttpHandlerFunc(fn)
}
