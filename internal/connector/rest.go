package connector

import (
	"errors"

	"github.com/Kucoin/kucoin-go-sdk"
	"github.com/milkywaybrain/cryptogalaxy/internal/config"
)

// REST is for REST connection.
type REST struct {
	KucoinService *kucoin.ApiService
}

var rest REST

// InitREST initializes http client with configured values.
func InitREST(cfg *config.REST) *REST {
	if rest.KucoinService == nil {
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
