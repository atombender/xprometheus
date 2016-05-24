# xprometheus

A Go library to transparently instrument [xmux](https://godoc.org/github.com/rs/xmux) routes with [Prometheus](https://prometheus.io/).

## Importing

```go
import "github.com/atombender/xprometheus"
```

## Example use

`xprometheus.WrapMux()` returns a new struct that has nearly exactly the same interface as `xmux.Mux`. Every handler will be instrumented with Prometheus.

```go
import (
  "github.com/prometheus/client_golang/prometheus"
  "github.com/rs/xhandler"
	"github.com/rs/xmux"
  "github.com/atombender/xprometheus"
)

func thingHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
  // ...
}

func main() {
  mux = mux.New()
  mux.Handle("GET", "/metrics", prometheus.Handler())

  pmux := xprometheus.WrapMux(mux.New(), prometheus.SummaryOpts{
  	Namespace: "thingapp",
  })
  pmux.GET("/api/v1/things", xhandler.HandlerFuncC(thingHandler))
  pmux.GET("/api/v1/things/:id", xhandler.HandlerFuncC(thingHandler))

  log.Fatal(http.ListenAndServe("locahost:8080", mux)
}
```
