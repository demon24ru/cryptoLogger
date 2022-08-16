package connector

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/milkywaybrain/cryptogalaxy/internal/config"
)

// REST is for REST connection.
type REST struct {
	HTTPClient *http.Client
}

var rest REST

// InitREST initializes http client with configured values.
func InitREST(cfg *config.REST) *REST {
	if rest.HTTPClient == nil {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.MaxIdleConns = cfg.MaxIdleConns
		t.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
		rest = REST{
			HTTPClient: &http.Client{
				Timeout:   time.Duration(cfg.ReqTimeoutSec) * time.Second,
				Transport: t,
			},
		}
	}
	return &rest
}

// GetREST returns already initialized http client.
func GetREST() (*REST, error) {
	if rest.HTTPClient == nil {
		return nil, errors.New("REST connection is not yet prepared")
	}
	return &rest, nil
}

// Request creates a new request object for http operation.
func (r *REST) Request(appCtx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(appCtx, method, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// Do makes GET http call to exchange.
func (r *REST) Do(req *http.Request) (*http.Response, error) {
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("code : %v, status : %v", resp.StatusCode, resp.Status)
	}
	return resp, nil
}
