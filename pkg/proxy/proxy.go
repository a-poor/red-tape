package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/charmbracelet/log"
	"gonum.org/v1/gonum/stat/distuv"
)

type ProxyConfig struct {
	// The destination URL to which the request should be proxied
	DestURL string

	// The probability of dropping a packet
	ProbDrop float64

	// The rate of packet delay (sampled from an exponential
	// distribution) before passing the request to the server
	PreDelayRate float64

	// The maximum delay before passing the request to the server
	PreDelayMax float64

	// The rate of packet delay after receiving the response from
	// the server, before passing it back to the client
	PostDelayRate float64

	// The maximum delay after receiving the response from the server,
	// before passing it back to the client
	PostDelayMax float64

	// An optional seed for the random number generator
	// (0 is treated as no seed)
	Seed uint64

	// If not set, http.DefaultTransport is used.
	Transport http.RoundTripper

	// Logger to use
	Logger log.Logger
}

func MakeRoundTripper(cfg *ProxyConfig) (http.RoundTripper, error) {
	// Get the logger...
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	// Get the transport or use the default...
	t := cfg.Transport
	if t == nil {
		t = http.DefaultTransport
	}

	// Create the pre- and post-request delay rng...
	preRng := distuv.Exponential{
		Rate: cfg.PreDelayRate,
	}
	postRng := distuv.Exponential{
		Rate: cfg.PostDelayRate,
	}

	// Create the pre- and post-request delay functions...
	preDelay := func() time.Duration {
		// If the rate is <= 0, return 0...
		if cfg.PreDelayRate <= 0 {
			return 0
		}

		// Otherwise, generate a random number...
		s := preRng.Rand()

		// If it's greater than the max, clamp it...
		if s > cfg.PreDelayMax {
			s = cfg.PreDelayMax
		}

		// Convert to milliseconds and return...
		return time.Duration(s) * time.Millisecond
	}
	postDelay := func() time.Duration {
		// If the rate is <= 0, return 0...
		if cfg.PostDelayRate <= 0 {
			return 0
		}

		// Otherwise, generate a random number...
		s := postRng.Rand()

		// If it's greater than the max, clamp it...
		if s > cfg.PostDelayMax {
			s = cfg.PostDelayMax
		}

		// Convert to milliseconds and return...
		return time.Duration(s) * time.Millisecond
	}

	// Return the http.RoundTripper...
	return roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		logger.Info("Incoming request")

		// Sleep before...
		d := preDelay()
		logger.Debug("Sleeping for %s before request.", d)
		time.Sleep(d)

		// Should the request be dropped?
		// TODO - Fill this in...

		// Send the request...
		logger.Debug("Sending request to %q", cfg.DestURL)
		resp, err := t.RoundTrip(r)

		// Sleep after...
		d = postDelay()
		logger.Debug("Sleeping for %s after response returned.", d)
		time.Sleep(d)

		// Return the results, unchanged...
		logger.Debug("Returning response to client.")
		return resp, err
	}), nil
}

func MakeProxy(cfg *ProxyConfig) (*httputil.ReverseProxy, error) {
	// Parse the configured proxy url...
	u, err := url.Parse(cfg.DestURL)
	if err != nil {
		return nil, err
	}

	// Create the http transport...
	rt, err := MakeRoundTripper(cfg)
	if err != nil {
		return nil, err
	}

	// Create the proxy and return...
	return &httputil.ReverseProxy{
		Transport: rt,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			r.Out.Host = r.In.Host
		},
	}, nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (rt roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}
