package main

import (
	"io"
	"net/http"
	"os"

	"github.com/juju/loggo"
	"github.com/urfave/cli/v2"
)

var logger = loggo.GetLogger("influxdb_http_auth_proxy")

// some code below is adapted from https://gist.github.com/yowu/f7dc34bd4736a65ff28d
// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

type handler struct {
	Upstream string
	Username string
	Password string
}

func (h *handler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	logger.Infof("Receive http request from %s (method: %s, url: %s)", req.RemoteAddr, req.Method, req.URL)

	client := &http.Client{}
	req.RequestURI = ""

	// update req url
	req.URL.Host = h.Upstream
	req.URL.Scheme = "http"

	for _, h := range hopHeaders {
		req.Header.Del(h)
	}

	// add auth query
	q := req.URL.Query()
	q.Add("u", h.Username)
	q.Add("p", h.Password)
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
		logger.Errorf("Got error from server:", err)
		return
	}
	defer resp.Body.Close()

	for _, h := range hopHeaders {
		resp.Header.Del(h)
	}

	for k, vv := range resp.Header {
		for _, v := range vv {
			wr.Header().Add(k, v)
		}
	}
	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
}

func action(c *cli.Context) error {
	addr := c.String("address")

	h := &handler{
		Upstream: c.String("upstream"),
		Username: c.String("username"),
		Password: c.String("password"),
	}

	logger.Infof("Listening at %s", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		logger.Errorf("Got error when listening: %s", err)
	}
	return nil
}

func main() {
	loggo.ConfigureLoggers("influxdb_http_auth_proxy=INFO")
	app := &cli.App{
		Name:    "influxdb-http-auth-proxy",
		Version: "1.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "address",
				Usage: "Listen address",
			},
			&cli.StringFlag{
				Name:  "upstream",
				Usage: "Upstream address",
			},
			&cli.StringFlag{
				Name:  "username",
				Usage: "InfluxDB username",
			},
			&cli.StringFlag{
				Name:  "password",
				Usage: "InfluxDB password",
			},
		},
		Action: action,
	}

	err := app.Run(os.Args)
	if err != nil {
		logger.Errorf("%s", err)
	}
}
