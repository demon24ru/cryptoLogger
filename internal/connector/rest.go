package connector

import (
	"errors"

	"github.com/Kucoin/kucoin-go-sdk"
	"github.com/milkywaybrain/cryptogalaxy/internal/config"
)

// REST is for REST connection.
type REST struct {
	//HTTPClient *http.Client
	KucoinService *kucoin.ApiService
}

var rest REST

// InitREST initializes http client with configured values.
func InitREST(cfg *config.REST) *REST {
	if rest.KucoinService == nil {
		//t := http.DefaultTransport.(*http.Transport).Clone()
		//t.MaxIdleConns = cfg.MaxIdleConns
		//t.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
		//rest = REST{
		//	HTTPClient: &http.Client{
		//		Timeout:   time.Duration(cfg.ReqTimeoutSec) * time.Second,
		//		Transport: t,
		//	},
		//}
		rest = REST{
			KucoinService: kucoin.NewApiService(
				kucoin.ApiKeyOption(cfg.KucoinKey),
				kucoin.ApiSecretOption(cfg.KucoinSecret),
				kucoin.ApiPassPhraseOption(cfg.KucoinPassphrase),
				kucoin.ApiKeyVersionOption("2"),
			),
		}
	}
	return &rest
}

// GetREST returns already initialized http client.
func GetREST() (*REST, error) {
	if rest.KucoinService == nil {
		return nil, errors.New("REST connection is not yet prepared")
	}
	return &rest, nil
}

//// Request creates a new request object for http operation.
//func (r *REST) Request(appCtx context.Context, method, url string) (*http.Request, error) {
//	//req, err := http.NewRequestWithContext(appCtx, method, url, http.NoBody)
//	//if err != nil {
//	//	return nil, err
//	//}
//	//return req, nil
//}
//
//// Do makes GET http call to exchange.
//func (r *REST) Do(req *http.Request) (*http.Response, error) {
//	resp, err := r.HTTPClient.Do(req)
//	if err != nil {
//		return nil, err
//	}
//	if resp.StatusCode != http.StatusOK {
//		return nil, fmt.Errorf("code : %v, status : %v", resp.StatusCode, resp.Status)
//	}
//	return resp, nil
//}
