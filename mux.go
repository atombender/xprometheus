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

func WrapMux(mux *xmux.Mux, opts prom.SummaryOpts) *Mux {
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

func (m *Mux) wrap(path string, handler xhandler.HandlerC) xhandler.HandlerC {
	labels := prom.Labels{}
	if m.opts.ConstLabels != nil {
		for k, v := range m.opts.ConstLabels {
			labels[k] = v
		}
	}
	labels["path"] = path

	opts := m.opts
	opts.ConstLabels = labels
	return InstrumentingHandlerWithOpts(opts, handler)
}
