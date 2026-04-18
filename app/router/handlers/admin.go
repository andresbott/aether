package handlers

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Admin() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		content := `<a href="/metrics">/metrics</a>`
		fmt.Fprint(w, content)
	})
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}
