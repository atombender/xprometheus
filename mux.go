package xprometheus

import (
	"net/http"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/rs/xhandler"
	"github.com/rs/xmux"
	"golang.org/x/net/context"
)

type Mux struct {
	mux  *xmux.Mux
	opts prom.SummaryOpts
}

// NewMux creates a new mux. It's like xmux.Mux, but automatically adds
// instrumentation to every registered route.
func NewMux() *Mux {
	return NewMuxWithOpts(prom.SummaryOpts{
		Subsystem: "http",
	})
}

// NewMuxWithOpts creates a new mux. It's like xmux.Mux, but automatically adds
// instrumentation to every registered route.
func NewMuxWithOpts(opts prom.SummaryOpts) *Mux {
	return Wrap(xmux.New(), opts)
}

// Wrap turns an xmux.Mux into an instrumented mux. It's like xmux.Mux, but
// automatically adds instrumentation to every registered route.
func Wrap(mux *xmux.Mux, opts prom.SummaryOpts) *Mux {
	if opts.Subsystem == "" {
		opts.Subsystem = "http"
	}
	return &Mux{mux, opts}
}

func (m *Mux) DELETE(path string, handler xhandler.HandlerC) {
	m.mux.DELETE(path, m.wrap(path, handler))
}

func (m *Mux) GET(path string, handler xhandler.HandlerC) {
	m.mux.GET(path, m.wrap(path, handler))
}

func (m *Mux) HEAD(path string, handler xhandler.HandlerC) {
	m.mux.HEAD(path, m.wrap(path, handler))
}

func (m *Mux) OPTIONS(path string, handler xhandler.HandlerC) {
	m.mux.OPTIONS(path, m.wrap(path, handler))
}

func (m *Mux) PATCH(path string, handler xhandler.HandlerC) {
	m.mux.PATCH(path, m.wrap(path, handler))
}

func (m *Mux) POST(path string, handler xhandler.HandlerC) {
	m.mux.POST(path, m.wrap(path, handler))
}

func (m *Mux) PUT(path string, handler xhandler.HandlerC) {
	m.mux.PUT(path, m.wrap(path, handler))
}

func (m *Mux) Handle(method, path string, handler http.Handler) {
	m.HandleC(method, path,
		xhandler.HandlerFuncC(func(_ context.Context, w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
		}),
	)
}

func (m *Mux) HandleC(method, path string, handler xhandler.HandlerC) {
	m.mux.HandleC(method, path, m.wrap(path, handler))
}

func (m *Mux) HandleFunc(method, path string, handler http.HandlerFunc) {
	m.HandleC(method, path,
		xhandler.HandlerFuncC(func(_ context.Context, w http.ResponseWriter, r *http.Request) {
			handler(w, r)
		}),
	)
}

func (m *Mux) HandleFuncC(method, path string, handler xhandler.HandlerFuncC) {
	m.HandleC(method, path, xhandler.HandlerFuncC(handler))
}

func (m *Mux) ServeHTTPC(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	m.mux.ServeHTTPC(ctx, w, r)
}

func (m *Mux) Lookup(method, path string) (xhandler.HandlerC, xmux.ParamHolder, bool) {
	return m.mux.Lookup(method, path)
}

func (m *Mux) wrap(path string, handler xhandler.HandlerC) xhandler.HandlerC {
	chain := xhandler.Chain{}
	chain.UseC(InstrumentingHandlerWithOpts(path, m.opts))
	return chain.HandlerC(handler)
}
