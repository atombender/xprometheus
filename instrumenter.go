package xprometheus

// Code adapted from github.com/prometheus/client_golang/blob/master/prometheus/http.go
// (copyright 2014 The Prometheus Authors).

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/rs/xhandler"
	"golang.org/x/net/context"
)

type nower interface {
	Now() time.Time
}

type nowFunc func() time.Time

func (n nowFunc) Now() time.Time {
	return n()
}

var now nower = nowFunc(func() time.Time {
	return time.Now()
})

// InstrumentingHandler returns an instrumenting handler with default
// options. The route can be empty, in which case the request URL path
// is used as the route label.
func InstrumentingHandler(route string) func(next xhandler.HandlerC) xhandler.HandlerC {
	return InstrumentingHandlerWithOpts(route,
		prom.SummaryOpts{
			Subsystem:   "http",
			ConstLabels: prom.Labels{},
		})
}

// InstrumentingHandlerWithOpts returns an instrumenting handler with
// custom summary options. The route can be empty, in which case the
// request URL path is used as the route label.
func InstrumentingHandlerWithOpts(
	route string,
	opts prom.SummaryOpts) func(next xhandler.HandlerC) xhandler.HandlerC {
	return func(next xhandler.HandlerC) xhandler.HandlerC {
		reqCnt := prom.NewCounterVec(
			prom.CounterOpts{
				Namespace:   opts.Namespace,
				Subsystem:   opts.Subsystem,
				Name:        "requests_total",
				Help:        "Total number of HTTP requests made.",
				ConstLabels: opts.ConstLabels,
			},
			[]string{"method", "code", "route"},
		)

		opts.Name = "request_duration_microseconds"
		opts.Help = "The HTTP request latencies in microseconds."
		reqDur := prom.NewSummaryVec(opts, []string{"route"})

		opts.Name = "request_size_bytes"
		opts.Help = "The HTTP request sizes in bytes."
		reqSz := prom.NewSummaryVec(opts, []string{"route"})

		opts.Name = "response_size_bytes"
		opts.Help = "The HTTP response sizes in bytes."
		resSz := prom.NewSummaryVec(opts, []string{"route"})

		regReqCnt := prom.MustRegisterOrGet(reqCnt).(*prom.CounterVec)
		regReqDur := prom.MustRegisterOrGet(reqDur).(*prom.SummaryVec)
		regReqSz := prom.MustRegisterOrGet(reqSz).(*prom.SummaryVec)
		regResSz := prom.MustRegisterOrGet(resSz).(*prom.SummaryVec)

		return xhandler.HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
			now := time.Now()

			delegate := &responseWriterDelegator{ResponseWriter: w}
			out := make(chan int)
			urlLen := 0
			if r.URL != nil {
				urlLen = len(r.URL.String())
			}
			go computeApproximateRequestSize(r, out, urlLen)

			_, cn := w.(http.CloseNotifier)
			_, fl := w.(http.Flusher)
			_, hj := w.(http.Hijacker)
			_, rf := w.(io.ReaderFrom)
			var newW http.ResponseWriter
			if cn && fl && hj && rf {
				newW = &fancyResponseWriterDelegator{delegate}
			} else {
				newW = delegate
			}

			next.ServeHTTPC(ctx, newW, r)

			elapsed := float64(time.Since(now)) / float64(time.Microsecond)

			method := sanitizeMethod(r.Method)
			code := sanitizeCode(delegate.status)
			routeLabel := route
			if routeLabel == "" {
				routeLabel = r.URL.Path
			}
			regReqCnt.WithLabelValues(method, code, routeLabel).Inc()
			regReqDur.WithLabelValues(routeLabel).Observe(elapsed)
			regResSz.WithLabelValues(routeLabel).Observe(float64(delegate.written))
			regReqSz.WithLabelValues(routeLabel).Observe(float64(<-out))
		})
	}
}

func computeApproximateRequestSize(r *http.Request, out chan int, s int) {
	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	// N.B. r.Form and r.MultipartForm are assumed to be included in r.URL.

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}
	out <- s
}

type responseWriterDelegator struct {
	http.ResponseWriter

	handler, method string
	status          int
	written         int64
	wroteHeader     bool
}

func (r *responseWriterDelegator) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *responseWriterDelegator) WriteHeader(code int) {
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseWriterDelegator) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

type fancyResponseWriterDelegator struct {
	*responseWriterDelegator
}

func (f *fancyResponseWriterDelegator) CloseNotify() <-chan bool {
	return f.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (f *fancyResponseWriterDelegator) Flush() {
	f.ResponseWriter.(http.Flusher).Flush()
}

func (f *fancyResponseWriterDelegator) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return f.ResponseWriter.(http.Hijacker).Hijack()
}

func (f *fancyResponseWriterDelegator) ReadFrom(r io.Reader) (int64, error) {
	if !f.wroteHeader {
		f.WriteHeader(http.StatusOK)
	}
	n, err := f.ResponseWriter.(io.ReaderFrom).ReadFrom(r)
	f.written += n
	return n, err
}

func sanitizeMethod(m string) string {
	switch m {
	case "GET", "get":
		return "get"
	case "PUT", "put":
		return "put"
	case "HEAD", "head":
		return "head"
	case "POST", "post":
		return "post"
	case "DELETE", "delete":
		return "delete"
	case "CONNECT", "connect":
		return "connect"
	case "OPTIONS", "options":
		return "options"
	case "NOTIFY", "notify":
		return "notify"
	default:
		return strings.ToLower(m)
	}
}

func sanitizeCode(s int) string {
	switch s {
	case 100:
		return "100"
	case 101:
		return "101"

	case 200:
		return "200"
	case 201:
		return "201"
	case 202:
		return "202"
	case 203:
		return "203"
	case 204:
		return "204"
	case 205:
		return "205"
	case 206:
		return "206"

	case 300:
		return "300"
	case 301:
		return "301"
	case 302:
		return "302"
	case 304:
		return "304"
	case 305:
		return "305"
	case 307:
		return "307"

	case 400:
		return "400"
	case 401:
		return "401"
	case 402:
		return "402"
	case 403:
		return "403"
	case 404:
		return "404"
	case 405:
		return "405"
	case 406:
		return "406"
	case 407:
		return "407"
	case 408:
		return "408"
	case 409:
		return "409"
	case 410:
		return "410"
	case 411:
		return "411"
	case 412:
		return "412"
	case 413:
		return "413"
	case 414:
		return "414"
	case 415:
		return "415"
	case 416:
		return "416"
	case 417:
		return "417"
	case 418:
		return "418"

	case 500:
		return "500"
	case 501:
		return "501"
	case 502:
		return "502"
	case 503:
		return "503"
	case 504:
		return "504"
	case 505:
		return "505"

	case 428:
		return "428"
	case 429:
		return "429"
	case 431:
		return "431"
	case 511:
		return "511"

	default:
		return strconv.Itoa(s)
	}
}
