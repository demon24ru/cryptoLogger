package initializer

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/milkywaybrain/cryptogalaxy/internal/config"
	"github.com/milkywaybrain/cryptogalaxy/internal/connector"
	"github.com/milkywaybrain/cryptogalaxy/internal/exchange"
	"github.com/milkywaybrain/cryptogalaxy/internal/storage"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"golang.org/x/sync/errgroup"
)

// Start will initialize various required systems and then execute the app.
func Start(mainCtx context.Context, cfg *config.Config) error {

	// Setting up logger.
	// If the path given in the config for logging ends with .log then create a log file with the same name and
	// write log messages to it. Otherwise, create a new log file with a timestamp attached to it's name in the given path.
	var (
		logFile *os.File
		err     error
	)
	if strings.HasSuffix(cfg.Log.FilePath, ".log") {
		logFile, err = os.OpenFile(cfg.Log.FilePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return fmt.Errorf("not able to open or create log file: %v", cfg.Log.FilePath)
		}
	} else {
		logFile, err = os.Create(cfg.Log.FilePath + "_" + strconv.Itoa(int(time.Now().Unix())) + ".log")
		if err != nil {
			return fmt.Errorf("not able to create log file: %v", cfg.Log.FilePath+"_"+strconv.Itoa(int(time.Now().Unix()))+".log")
		}
	}
	defer func() {
		log.Error().Msg("exiting the app")
		_ = logFile.Close()
	}()

	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	switch cfg.Log.Level {
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	fileLogger := zerolog.New(logFile).With().Timestamp().Logger()
	log.Logger = fileLogger
	log.Info().Msg("logger setup is done")

	// Establish connections to different storage systems, connectors and
	// also validate few user defined config values.
	var (
		restConn      bool
		terStr        bool
		clickHouseStr bool
	)
	for _, exch := range cfg.Exchanges {
		for _, market := range exch.Markets {
			for _, info := range market.Info {
				for _, str := range info.Storages {
					switch str {
					case "terminal":
						if !terStr {
							_ = storage.InitTerminal(os.Stdout)
							terStr = true
							log.Info().Msg("terminal connected")
						}
					case "clickhouse":
						if !clickHouseStr {
							_, err = storage.InitClickHouse(&cfg.Connection.ClickHouse)
							if err != nil {
								err = errors.Wrap(err, "clickhouse connection")
								log.Error().Stack().Err(errors.WithStack(err)).Msg("")
								return err
							}
							clickHouseStr = true
							log.Info().Msg("clickhouse connected")
						}
					}
				}
				if info.Connector == "rest" {
					if !restConn {
						_ = connector.InitREST(&cfg.Connection.REST)
						restConn = true
					}
					if info.RESTPingIntSec < 1 {
						err = errors.New("rest_ping_interval_sec should be greater than zero")
						log.Error().Stack().Err(errors.WithStack(err)).Msg("")
						return err
					}
				}
			}
		}
	}

	// Start each exchange function. If any exchange fails after retry, force all the other exchanges to stop and
	// exit the app.
	appErrGroup, appCtx := errgroup.WithContext(mainCtx)

	for _, exch := range cfg.Exchanges {
		markets := exch.Markets
		retry := exch.Retry
		switch exch.Name {
		//case "ftx":
		//	appErrGroup.Go(func() error {
		//		return exchange.StartFtx(appCtx, markets, &retry, &cfg.Connection)
		//	})
		//case "coinbase-pro":
		//	appErrGroup.Go(func() error {
		//		return exchange.StartCoinbasePro(appCtx, markets, &retry, &cfg.Connection)
		//	})
		case "binance":
			appErrGroup.Go(func() error {
				return exchange.StartBinance(appCtx, markets, &retry, &cfg.Connection)
			})
		case "binanceFutures":
			appErrGroup.Go(func() error {
				return exchange.StartBinanceFutures(appCtx, markets, &retry, &cfg.Connection)
			})
		//case "bitfinex":
		//	appErrGroup.Go(func() error {
		//		return exchange.StartBitfinex(appCtx, markets, &retry, &cfg.Connection)
		//	})
		//case "huobi":
		//	appErrGroup.Go(func() error {
		//		return exchange.StartHuobi(appCtx, markets, &retry, &cfg.Connection)
		//	})
		//case "gateio":
		//	appErrGroup.Go(func() error {
		//		return exchange.StartGateio(appCtx, markets, &retry, &cfg.Connection)
		//	})
		case "kucoin":
			appErrGroup.Go(func() error {
				return exchange.StartKucoin(appCtx, markets, &retry, &cfg.Connection)
			})
		case "kucoinFutures":
			appErrGroup.Go(func() error {
				return exchange.StartKucoinFutures(appCtx, markets, &retry, &cfg.Connection)
			})
			//case "bitstamp":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartBitstamp(appCtx, markets, &retry, &cfg.Connection)
			//	})
		case "bybit":
			appErrGroup.Go(func() error {
				return exchange.StartByBit(appCtx, markets, &retry, &cfg.Connection)
			})
		case "bybitFutures":
			appErrGroup.Go(func() error {
				return exchange.StartByBitFutures(appCtx, markets, &retry, &cfg.Connection)
			})
			//case "probit":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartProbit(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "gemini":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartGemini(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "bitmart":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartBitmart(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "digifinex":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartDigifinex(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "ascendex":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartAscendex(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "kraken":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartKraken(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "binance-us":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartBinanceUS(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "okex":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartOKEx(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "ftx-us":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartFtxUS(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "hitbtc":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartHitBTC(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "aax":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartAAX(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "bitrue":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartBitrue(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "btse":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartBTSE(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "mexo":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartMexo(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "bequant":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartBequant(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "lbank":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartLBank(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "coinflex":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartCoinFlex(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "binance-tr":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartBinanceTR(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "cryptodot-com":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartCryptodotCom(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "fmfwio":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartFmfwio(appCtx, markets, &retry, &cfg.Connection)
			//	})
			//case "changelly-pro":
			//	appErrGroup.Go(func() error {
			//		return exchange.StartChangellyPro(appCtx, markets, &retry, &cfg.Connection)
			//	})
		}
	}
	log.Error().Msg("start app")

	err = appErrGroup.Wait()
	if err != nil {
		return err
	}
	return nil
}
