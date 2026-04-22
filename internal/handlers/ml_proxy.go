package handlers

import (
	"io"
	"net/http"
	"net/url"
)

// MLProxyHandler proxies requests to the Python ML API running on port 5000.
func MLProxyHandler(mlAPIURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Enable CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		target, err := url.Parse(mlAPIURL + r.URL.Path)
		if err != nil {
			http.Error(w, "invalid ML API URL", http.StatusInternalServerError)
			return
		}
		target.RawQuery = r.URL.RawQuery

		req, err := http.NewRequest(r.Method, target.String(), r.Body)
		if err != nil {
			http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
			return
		}
		req.Header = r.Header

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "failed to reach ML API (is it running?)", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
