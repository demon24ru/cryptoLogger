package test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/ClickHouse/clickhouse-go"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	_ "github.com/go-sql-driver/mysql"
	jsoniter "github.com/json-iterator/go"
	"github.com/milkywaybrain/cryptogalaxy/internal/config"
	"github.com/milkywaybrain/cryptogalaxy/internal/initializer"
	"github.com/milkywaybrain/cryptogalaxy/internal/storage"
	nc "github.com/nats-io/nats.go"
	"golang.org/x/sync/errgroup"
)

// TestCryptogalaxy is a combination of unit and integration test for the app.
func TestCryptogalaxy(t *testing.T) {

	// Load config file values.
	cfgPath := "./config_test.json"
	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		t.Log("ERROR : Not able to find config file :", cfgPath)
		t.FailNow()
	}
	var cfg config.Config
	if err = jsoniter.NewDecoder(cfgFile).Decode(&cfg); err != nil {
		t.Log("ERROR : Not able to parse JSON from config file :", cfgPath)
		t.FailNow()
	}
	cfgFile.Close()

	// Get the details of different storage options configured for testing.
	var (
		terStr        bool
		sqlStr        bool
		esStr         bool
		influxStr     bool
		natsStr       bool
		clickHouseStr bool
		s3Str         bool

		fOutFile   *os.File
		mysql      *storage.MySQL
		es         *storage.ElasticSearch
		influx     *storage.InfluxDB
		clickhouse *storage.ClickHouse
		s3         *storage.S3
		nOutFile   *os.File
	)
	enabledExchanges := make(map[string]bool)
	for _, exch := range cfg.Exchanges {
		enabledExchanges[exch.Name] = true
		for _, market := range exch.Markets {
			for _, info := range market.Info {
				for _, str := range info.Storages {
					switch str {
					case "terminal":
						if !terStr {
							terStr = true
						}
					case "mysql":
						if !sqlStr {
							sqlStr = true
						}
					case "elastic_search":
						if !esStr {
							esStr = true
						}
					case "influxdb":
						if !influxStr {
							influxStr = true
						}
					case "nats":
						if !natsStr {
							natsStr = true
						}
					case "clickhouse":
						if !clickHouseStr {
							clickHouseStr = true
						}
					case "s3":
						if !s3Str {
							s3Str = true
						}
					}
				}
				if info.Connector == "rest" {
					if info.RESTPingIntSec < 1 {
						t.Log("ERROR : rest_ping_interval_sec should be greater than zero")
						t.FailNow()
					}
				}
			}
		}
	}

	// For testing, mysql schema name should start with the name test to avoid mistakenly messing up with production one.
	if sqlStr && !strings.HasPrefix(cfg.Connection.MySQL.Schema, "test") {
		t.Log("ERROR : mysql schema name should start with test for testing")
		t.FailNow()
	}

	// For testing, elastic search index name should start with the name test to avoid mistakenly messing up with
	// production one.
	if esStr && !strings.HasPrefix(cfg.Connection.ES.IndexName, "test") {
		t.Log("ERROR : elastic search index name should start with test for testing")
		t.FailNow()
	}

	// For testing, influxdb bucket name should start with the name test to avoid mistakenly messing up with
	// production one.
	if influxStr && !strings.HasPrefix(cfg.Connection.InfluxDB.Bucket, "test") {
		t.Log("ERROR : influxdb bucket name should start with test for testing")
		t.FailNow()
	}

	// For testing, nats subject name should start with the name test to avoid mistakenly messing up with
	// production one.
	if natsStr && !strings.HasPrefix(cfg.Connection.NATS.SubjectBaseName, "test") {
		t.Log("ERROR : nats subject base name should start with test for testing")
		t.FailNow()
	}

	// For testing, clickhouse schema name should start with the name test to avoid mistakenly messing up with production one.
	if clickHouseStr && !strings.HasPrefix(cfg.Connection.ClickHouse.Schema, "test") {
		t.Log("ERROR : clickhouse schema name should start with test for testing")
		t.FailNow()
	}

	// For testing, s3 bucket name should start with the name test to avoid mistakenly messing up with production one.
	if s3Str && !strings.HasPrefix(cfg.Connection.S3.Bucket, "test") {
		t.Log("ERROR : s3 bucket name should start with test for testing")
		t.FailNow()
	}

	// Terminal output we can't actually test, so make file as terminal output.
	if terStr {
		fOutFile, err = os.Create("./data_test/ter_storage_test.txt")
		if err != nil {
			t.Log("ERROR : not able to create test terminal storage file : ./data_test/ter_storage_test.txt")
			t.FailNow()
		}
		_ = storage.InitTerminal(fOutFile)
	}

	// Delete all data from mysql to have fresh one.
	if sqlStr {
		mysql, err = storage.InitMySQL(&cfg.Connection.MySQL)
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}

		var mysqlCtx context.Context
		if cfg.Connection.MySQL.ReqTimeoutSec > 0 {
			timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Connection.MySQL.ReqTimeoutSec)*time.Second)
			mysqlCtx = timeoutCtx
			defer cancel()
		} else {
			mysqlCtx = context.Background()
		}
		_, err = mysql.DB.ExecContext(mysqlCtx, "truncate ticker")
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}

		if cfg.Connection.MySQL.ReqTimeoutSec > 0 {
			timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Connection.MySQL.ReqTimeoutSec)*time.Second)
			mysqlCtx = timeoutCtx
			defer cancel()
		} else {
			mysqlCtx = context.Background()
		}
		_, err = mysql.DB.ExecContext(mysqlCtx, "truncate trade")
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}
	}

	// Delete all data from elastic search to have fresh one.
	if esStr {
		es, err = storage.InitElasticSearch(&cfg.Connection.ES)
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}

		indexNames := make([]string, 0, 1)
		indexNames = append(indexNames, es.IndexName)
		var buf bytes.Buffer
		deleteQuery := []byte(`{"query":{"match_all":{}}}`)
		buf.Grow(len(deleteQuery))
		buf.Write(deleteQuery)

		var esCtx context.Context
		if cfg.Connection.ES.ReqTimeoutSec > 0 {
			timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Connection.ES.ReqTimeoutSec)*time.Second)
			esCtx = timeoutCtx
			defer cancel()
		} else {
			esCtx = context.Background()
		}
		resp, esErr := es.ES.DeleteByQuery(indexNames, bytes.NewReader(buf.Bytes()), es.ES.DeleteByQuery.WithContext(esCtx))
		if esErr != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Log("ERROR : " + fmt.Errorf("code : %v, status : %v", resp.StatusCode, resp.Status()).Error())
			t.FailNow()
		}
		_, err = io.Copy(io.Discard, resp.Body)
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}
	}

	// Delete all data from influxdb to have fresh one.
	if influxStr {
		influx, err = storage.InitInfluxDB(&cfg.Connection.InfluxDB)
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}

		var influxCtx context.Context
		if cfg.Connection.InfluxDB.ReqTimeoutSec > 0 {
			timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Connection.InfluxDB.ReqTimeoutSec)*time.Second)
			influxCtx = timeoutCtx
			defer cancel()
		} else {
			influxCtx = context.Background()
		}
		start := time.Date(2021, time.Month(7), 25, 1, 1, 1, 1, time.UTC)
		end := time.Now().UTC()
		err = influx.DeleteAPI.DeleteWithName(influxCtx, cfg.Connection.InfluxDB.Organization, cfg.Connection.InfluxDB.Bucket, start, end, "")
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}
	}

	// Subscribe to NATS subject. Write received data to a file.
	if natsStr {
		nOutFile, err = os.Create("./data_test/nats_storage_test.txt")
		if err != nil {
			t.Log("ERROR : not able to create test nats storage file : ./data_test/nats_storage_test.txt")
			t.FailNow()
		}
		nats, natsErr := storage.InitNATS(&cfg.Connection.NATS)
		if natsErr != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}
		go natsSub(cfg.Connection.NATS.SubjectBaseName+".*", nOutFile, nats, t)
	}

	// Delete all data from clickhouse to have fresh one.
	if clickHouseStr {
		clickhouse, err = storage.InitClickHouse(&cfg.Connection.ClickHouse)
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}

		_, err = clickhouse.DB.Exec("truncate ticker")
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}

		_, err = clickhouse.DB.Exec("truncate trade")
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}
	}

	// Delete all data from s3 to have fresh one.
	if s3Str {
		s3, err = storage.InitS3(&cfg.Connection.S3)
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}

		for {
			objInput := &awss3.ListObjectsV2Input{
				Bucket: &cfg.Connection.S3.Bucket,
			}
			objResp, errs3 := s3.Client.ListObjectsV2(context.TODO(), objInput)
			if errs3 != nil {
				t.Log("ERROR : " + errs3.Error())
				t.FailNow()
			}
			if len(objResp.Contents) > 0 {
				delArray := make([]types.ObjectIdentifier, 0, len(objResp.Contents))
				for _, item := range objResp.Contents {
					objIdent := types.ObjectIdentifier{
						Key: item.Key,
					}
					delArray = append(delArray, objIdent)
					if len(delArray) == 1000 {
						deleteDetail := types.Delete{
							Objects: delArray,
						}
						delInput := &awss3.DeleteObjectsInput{
							Bucket: &cfg.Connection.S3.Bucket,
							Delete: &deleteDetail,
						}
						_, err = s3.Client.DeleteObjects(context.TODO(), delInput)
						if err != nil {
							t.Log("ERROR : " + err.Error())
							t.FailNow()
						}
						delArray = nil
					}
				}
				if len(delArray) > 0 {
					deleteDetail := types.Delete{
						Objects: delArray,
					}
					delInput := &awss3.DeleteObjectsInput{
						Bucket: &cfg.Connection.S3.Bucket,
						Delete: &deleteDetail,
					}
					_, err = s3.Client.DeleteObjects(context.TODO(), delInput)
					if err != nil {
						t.Log("ERROR : " + err.Error())
						t.FailNow()
					}
				}
			} else {
				break
			}
		}
	}

	// Execute the app for 2 minute, which is good enough time to get the data from exchanges.
	// After that cancel app execution through context error.
	// If there is any actual error from app execution, then stop testing.
	testErrGroup, testCtx := errgroup.WithContext(context.Background())

	testErrGroup.Go(func() error {
		return initializer.Start(testCtx, &cfg)
	})

	t.Log("INFO : Executing app for 2 minute to get the data from exchanges.")
	testErrGroup.Go(func() error {
		tick := time.NewTicker(2 * time.Minute)
		defer tick.Stop()
		select {
		case <-tick.C:
			return errors.New("canceling app execution and starting data test")
		case <-testCtx.Done():
			return testCtx.Err()
		}
	})

	err = testErrGroup.Wait()
	if err != nil {
		if err.Error() == "canceling app execution and starting data test" {
			t.Log("INFO : " + err.Error())
		} else {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}
	}

	// Close file which has been set as the terminal output in the previous step.
	if terStr {
		err = fOutFile.Close()
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}
	}

	// Close file which has been set as the nats output in the previous step.
	if natsStr {
		err = nOutFile.Close()
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}
	}

	// Read data from different storage which has been set in the app execution stage.
	// Then verify it.

	// Read from S3 once for all exchanges.
	s3AllTickers := make(map[string]map[string]storage.Ticker)
	s3AllTrades := make(map[string]map[string]storage.Trade)
	if s3Str {
		err = readS3(s3AllTickers, s3AllTrades, s3)
		if err != nil {
			t.Log("ERROR : " + err.Error())
			t.FailNow()
		}
	}

	// FTX exchange.
	var ftxFail bool

	terTickers := make(map[string]storage.Ticker)
	terTrades := make(map[string]storage.Trade)
	mysqlTickers := make(map[string]storage.Ticker)
	mysqlTrades := make(map[string]storage.Trade)
	esTickers := make(map[string]storage.Ticker)
	esTrades := make(map[string]storage.Trade)
	influxTickers := make(map[string]storage.Ticker)
	influxTrades := make(map[string]storage.Trade)
	natsTickers := make(map[string]storage.Ticker)
	natsTrades := make(map[string]storage.Trade)
	clickHouseTickers := make(map[string]storage.Ticker)
	clickHouseTrades := make(map[string]storage.Trade)
	s3Tickers := s3AllTickers["ftx"]
	s3Trades := s3AllTrades["ftx"]

	if enabledExchanges["ftx"] {
		if terStr {
			err = readTerminal("ftx", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx exchange function")
				ftxFail = true
			}
		}

		if sqlStr && !ftxFail {
			err = readMySQL("ftx", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx exchange function")
				ftxFail = true
			}
		}

		if esStr && !ftxFail {
			err = readElasticSearch("ftx", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx exchange function")
				ftxFail = true
			}
		}

		if influxStr && !ftxFail {
			err = readInfluxDB("ftx", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx exchange function")
				ftxFail = true
			}
		}

		if natsStr && !ftxFail {
			err = readNATS("ftx", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx exchange function")
				ftxFail = true
			}
		}

		if clickHouseStr && !ftxFail {
			err = readClickHouse("ftx", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx exchange function")
				ftxFail = true
			}
		}

		if !ftxFail {
			err = verifyData("ftx", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx exchange function")
				ftxFail = true
			} else {
				t.Log("SUCCESS : ftx exchange function")
			}
		}
	}

	// Coinbase-Pro exchange.
	var coinbaseProFail bool

	if enabledExchanges["coinbase-pro"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["coinbase-pro"]
		s3Trades = s3AllTrades["coinbase-pro"]

		if terStr {
			err = readTerminal("coinbase-pro", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinbase-pro exchange function")
				coinbaseProFail = true
			}
		}

		if sqlStr && !coinbaseProFail {
			err = readMySQL("coinbase-pro", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinbase-pro exchange function")
				coinbaseProFail = true
			}
		}

		if esStr && !coinbaseProFail {
			err = readElasticSearch("coinbase-pro", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinbase-pro exchange function")
				coinbaseProFail = true
			}
		}

		if influxStr && !coinbaseProFail {
			err = readInfluxDB("coinbase-pro", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinbase-pro exchange function")
				coinbaseProFail = true
			}
		}

		if natsStr && !coinbaseProFail {
			err = readNATS("coinbase-pro", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinbase-pro exchange function")
				coinbaseProFail = true
			}
		}

		if clickHouseStr && !coinbaseProFail {
			err = readClickHouse("coinbase-pro", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinbase-pro exchange function")
				coinbaseProFail = true
			}
		}

		if !coinbaseProFail {
			err = verifyData("ftx", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinbase-pro exchange function")
				coinbaseProFail = true
			} else {
				t.Log("SUCCESS : coinbase-pro exchange function")
			}
		}
	}

	// Binance exchange.
	var binanceFail bool

	if enabledExchanges["binance"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["binance"]
		s3Trades = s3AllTrades["binance"]

		if terStr {
			err = readTerminal("binance", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance exchange function")
				binanceFail = true
			}
		}

		if sqlStr && !binanceFail {
			err = readMySQL("binance", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance exchange function")
				binanceFail = true
			}
		}

		if esStr && !binanceFail {
			err = readElasticSearch("binance", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance exchange function")
				binanceFail = true
			}
		}

		if influxStr && !binanceFail {
			err = readInfluxDB("binance", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance exchange function")
				binanceFail = true
			}
		}

		if natsStr && !binanceFail {
			err = readNATS("binance", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance exchange function")
				binanceFail = true
			}
		}

		if clickHouseStr && !binanceFail {
			err = readClickHouse("binance", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance exchange function")
				binanceFail = true
			}
		}

		if !binanceFail {
			err = verifyData("binance", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance exchange function")
				binanceFail = true
			} else {
				t.Log("SUCCESS : binance exchange function")
			}
		}
	}

	// Bitfinex exchange.
	var bitfinexFail bool

	if enabledExchanges["bitfinex"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["bitfinex"]
		s3Trades = s3AllTrades["bitfinex"]

		if terStr {
			err = readTerminal("bitfinex", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitfinex exchange function")
				bitfinexFail = true
			}
		}

		if sqlStr && !bitfinexFail {
			err = readMySQL("bitfinex", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitfinex exchange function")
				bitfinexFail = true
			}
		}

		if esStr && !bitfinexFail {
			err = readElasticSearch("bitfinex", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitfinex exchange function")
				bitfinexFail = true
			}
		}

		if influxStr && !bitfinexFail {
			err = readInfluxDB("bitfinex", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitfinex exchange function")
				bitfinexFail = true
			}
		}

		if natsStr && !bitfinexFail {
			err = readNATS("bitfinex", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitfinex exchange function")
				bitfinexFail = true
			}
		}

		if clickHouseStr && !bitfinexFail {
			err = readClickHouse("bitfinex", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitfinex exchange function")
				bitfinexFail = true
			}
		}

		if !bitfinexFail {
			err = verifyData("bitfinex", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitfinex exchange function")
				bitfinexFail = true
			} else {
				t.Log("SUCCESS : bitfinex exchange function")
			}
		}
	}

	// Huobi exchange.
	var huobiFail bool

	if enabledExchanges["huobi"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["huobi"]
		s3Trades = s3AllTrades["huobi"]

		if terStr {
			err = readTerminal("huobi", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : huobi exchange function")
				huobiFail = true
			}
		}

		if sqlStr && !huobiFail {
			err = readMySQL("huobi", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : huobi exchange function")
				huobiFail = true
			}
		}

		if esStr && !huobiFail {
			err = readElasticSearch("huobi", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : huobi exchange function")
				huobiFail = true
			}
		}

		if influxStr && !huobiFail {
			err = readInfluxDB("huobi", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : huobi exchange function")
				huobiFail = true
			}
		}

		if natsStr && !huobiFail {
			err = readNATS("huobi", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : huobi exchange function")
				huobiFail = true
			}
		}

		if clickHouseStr && !huobiFail {
			err = readClickHouse("huobi", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : huobi exchange function")
				huobiFail = true
			}
		}

		if !huobiFail {
			err = verifyData("huobi", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : huobi exchange function")
				huobiFail = true
			} else {
				t.Log("SUCCESS : huobi exchange function")
			}
		}
	}

	// Gateio exchange.
	var gateioFail bool

	if enabledExchanges["gateio"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["gateio"]
		s3Trades = s3AllTrades["gateio"]

		if terStr {
			err = readTerminal("gateio", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gateio exchange function")
				gateioFail = true
			}
		}

		if sqlStr && !gateioFail {
			err = readMySQL("gateio", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gateio exchange function")
				gateioFail = true
			}
		}

		if esStr && !gateioFail {
			err = readElasticSearch("gateio", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gateio exchange function")
				gateioFail = true
			}
		}

		if influxStr && !gateioFail {
			err = readInfluxDB("gateio", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gateio exchange function")
				gateioFail = true
			}
		}

		if natsStr && !gateioFail {
			err = readNATS("gateio", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gateio exchange function")
				gateioFail = true
			}
		}

		if clickHouseStr && !gateioFail {
			err = readClickHouse("gateio", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gateio exchange function")
				gateioFail = true
			}
		}

		if !gateioFail {
			err = verifyData("gateio", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gateio exchange function")
				gateioFail = true
			} else {
				t.Log("SUCCESS : gateio exchange function")
			}
		}
	}

	// Kucoin exchange.
	var kucoinFail bool

	if enabledExchanges["kucoin"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["kucoin"]
		s3Trades = s3AllTrades["kucoin"]

		if terStr {
			err = readTerminal("kucoin", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kucoin exchange function")
				kucoinFail = true
			}
		}

		if sqlStr && !kucoinFail {
			err = readMySQL("kucoin", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kucoin exchange function")
				kucoinFail = true
			}
		}

		if esStr && !kucoinFail {
			err = readElasticSearch("kucoin", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kucoin exchange function")
				kucoinFail = true
			}
		}

		if influxStr && !kucoinFail {
			err = readInfluxDB("kucoin", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kucoin exchange function")
				kucoinFail = true
			}
		}

		if natsStr && !kucoinFail {
			err = readNATS("kucoin", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kucoin exchange function")
				kucoinFail = true
			}
		}

		if clickHouseStr && !kucoinFail {
			err = readClickHouse("kucoin", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kucoin exchange function")
				kucoinFail = true
			}
		}

		if !kucoinFail {
			err = verifyData("kucoin", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kucoin exchange function")
				kucoinFail = true
			} else {
				t.Log("SUCCESS : kucoin exchange function")
			}
		}
	}

	// Bitstamp exchange.
	var bitstampFail bool

	if enabledExchanges["bitstamp"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["bitstamp"]
		s3Trades = s3AllTrades["bitstamp"]

		if terStr {
			err = readTerminal("bitstamp", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitstamp exchange function")
				bitstampFail = true
			}
		}

		if sqlStr && !bitstampFail {
			err = readMySQL("bitstamp", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitstamp exchange function")
				bitstampFail = true
			}
		}

		if esStr && !bitstampFail {
			err = readElasticSearch("bitstamp", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitstamp exchange function")
				bitstampFail = true
			}
		}

		if influxStr && !bitstampFail {
			err = readInfluxDB("bitstamp", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitstamp exchange function")
				bitstampFail = true
			}
		}

		if natsStr && !bitstampFail {
			err = readNATS("bitstamp", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitstamp exchange function")
				bitstampFail = true
			}
		}

		if clickHouseStr && !bitstampFail {
			err = readClickHouse("bitstamp", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitstamp exchange function")
				bitstampFail = true
			}
		}

		if !bitstampFail {
			err = verifyData("bitstamp", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitstamp exchange function")
				bitstampFail = true
			} else {
				t.Log("SUCCESS : bitstamp exchange function")
			}
		}
	}

	// Bybit exchange.
	var bybitFail bool

	if enabledExchanges["bybit"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["bybit"]
		s3Trades = s3AllTrades["bybit"]

		if terStr {
			err = readTerminal("bybit", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bybit exchange function")
				bybitFail = true
			}
		}

		if sqlStr && !bybitFail {
			err = readMySQL("bybit", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bybit exchange function")
				bybitFail = true
			}
		}

		if esStr && !bybitFail {
			err = readElasticSearch("bybit", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bybit exchange function")
				bybitFail = true
			}
		}

		if influxStr && !bybitFail {
			err = readInfluxDB("bybit", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bybit exchange function")
				bybitFail = true
			}
		}

		if natsStr && !bybitFail {
			err = readNATS("bybit", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bybit exchange function")
				bybitFail = true
			}
		}

		if clickHouseStr && !bybitFail {
			err = readClickHouse("bybit", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bybit exchange function")
				bybitFail = true
			}
		}

		if !bybitFail {
			err = verifyData("bybit", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bybit exchange function")
				bybitFail = true
			} else {
				t.Log("SUCCESS : bybit exchange function")
			}
		}
	}

	// Probit exchange.
	var probitFail bool

	if enabledExchanges["probit"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["probit"]
		s3Trades = s3AllTrades["probit"]

		if terStr {
			err = readTerminal("probit", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : probit exchange function")
				probitFail = true
			}
		}

		if sqlStr && !probitFail {
			err = readMySQL("probit", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : probit exchange function")
				probitFail = true
			}
		}

		if esStr && !probitFail {
			err = readElasticSearch("probit", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : probit exchange function")
				probitFail = true
			}
		}

		if influxStr && !probitFail {
			err = readInfluxDB("probit", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : probit exchange function")
				probitFail = true
			}
		}

		if natsStr && !probitFail {
			err = readNATS("probit", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : probit exchange function")
				probitFail = true
			}
		}

		if clickHouseStr && !probitFail {
			err = readClickHouse("probit", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : probit exchange function")
				probitFail = true
			}
		}

		if !probitFail {
			err = verifyData("probit", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : probit exchange function")
				probitFail = true
			} else {
				t.Log("SUCCESS : probit exchange function")
			}
		}
	}

	// Gemini exchange.
	var geminiFail bool

	if enabledExchanges["gemini"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["gemini"]
		s3Trades = s3AllTrades["gemini"]

		if terStr {
			err = readTerminal("gemini", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gemini exchange function")
				geminiFail = true
			}
		}

		if sqlStr && !geminiFail {
			err = readMySQL("gemini", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gemini exchange function")
				geminiFail = true
			}
		}

		if esStr && !geminiFail {
			err = readElasticSearch("gemini", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gemini exchange function")
				geminiFail = true
			}
		}

		if influxStr && !geminiFail {
			err = readInfluxDB("gemini", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gemini exchange function")
				geminiFail = true
			}
		}

		if natsStr && !geminiFail {
			err = readNATS("gemini", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gemini exchange function")
				geminiFail = true
			}
		}

		if clickHouseStr && !geminiFail {
			err = readClickHouse("gemini", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gemini exchange function")
				geminiFail = true
			}
		}

		if !geminiFail {
			err = verifyData("gemini", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : gemini exchange function")
				geminiFail = true
			} else {
				t.Log("SUCCESS : gemini exchange function")
			}
		}
	}

	// Bitmart exchange.
	var bitmartFail bool

	if enabledExchanges["bitmart"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["bitmart"]
		s3Trades = s3AllTrades["bitmart"]

		if terStr {
			err = readTerminal("bitmart", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitmart exchange function")
				bitmartFail = true
			}
		}

		if sqlStr && !bitmartFail {
			err = readMySQL("bitmart", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitmart exchange function")
				bitmartFail = true
			}
		}

		if esStr && !bitmartFail {
			err = readElasticSearch("bitmart", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitmart exchange function")
				bitmartFail = true
			}
		}

		if influxStr && !bitmartFail {
			err = readInfluxDB("bitmart", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitmart exchange function")
				bitmartFail = true
			}
		}

		if natsStr && !bitmartFail {
			err = readNATS("bitmart", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitmart exchange function")
				bitmartFail = true
			}
		}

		if clickHouseStr && !bitmartFail {
			err = readClickHouse("bitmart", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitmart exchange function")
				bitmartFail = true
			}
		}

		if !bitmartFail {
			err = verifyData("bitmart", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitmart exchange function")
				bitmartFail = true
			} else {
				t.Log("SUCCESS : bitmart exchange function")
			}
		}
	}

	// Digifinex exchange.
	var digifinexFail bool

	if enabledExchanges["digifinex"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["digifinex"]
		s3Trades = s3AllTrades["digifinex"]

		if terStr {
			err = readTerminal("digifinex", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : digifinex exchange function")
				digifinexFail = true
			}
		}

		if sqlStr && !digifinexFail {
			err = readMySQL("digifinex", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : digifinex exchange function")
				digifinexFail = true
			}
		}

		if esStr && !digifinexFail {
			err = readElasticSearch("digifinex", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : digifinex exchange function")
				digifinexFail = true
			}
		}

		if influxStr && !digifinexFail {
			err = readInfluxDB("digifinex", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : digifinex exchange function")
				digifinexFail = true
			}
		}

		if natsStr && !digifinexFail {
			err = readNATS("digifinex", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : digifinex exchange function")
				digifinexFail = true
			}
		}

		if clickHouseStr && !digifinexFail {
			err = readClickHouse("digifinex", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : digifinex exchange function")
				digifinexFail = true
			}
		}

		if !digifinexFail {
			err = verifyData("digifinex", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : digifinex exchange function")
				digifinexFail = true
			} else {
				t.Log("SUCCESS : digifinex exchange function")
			}
		}
	}

	// Ascendex exchange.
	var ascendexFail bool

	if enabledExchanges["ascendex"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["ascendex"]
		s3Trades = s3AllTrades["ascendex"]

		if terStr {
			err = readTerminal("ascendex", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ascendex exchange function")
				ascendexFail = true
			}
		}

		if sqlStr && !ascendexFail {
			err = readMySQL("ascendex", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ascendex exchange function")
				ascendexFail = true
			}
		}

		if esStr && !ascendexFail {
			err = readElasticSearch("ascendex", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ascendex exchange function")
				ascendexFail = true
			}
		}

		if influxStr && !ascendexFail {
			err = readInfluxDB("ascendex", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ascendex exchange function")
				ascendexFail = true
			}
		}

		if natsStr && !ascendexFail {
			err = readNATS("ascendex", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ascendex exchange function")
				ascendexFail = true
			}
		}

		if clickHouseStr && !ascendexFail {
			err = readClickHouse("ascendex", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ascendex exchange function")
				ascendexFail = true
			}
		}

		if !ascendexFail {
			err = verifyData("ascendex", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ascendex exchange function")
				ascendexFail = true
			} else {
				t.Log("SUCCESS : ascendex exchange function")
			}
		}
	}

	// Kraken exchange.
	var krakenFail bool

	if enabledExchanges["kraken"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["kraken"]
		s3Trades = s3AllTrades["kraken"]

		if terStr {
			err = readTerminal("kraken", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kraken exchange function")
				krakenFail = true
			}
		}

		if sqlStr && !krakenFail {
			err = readMySQL("kraken", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kraken exchange function")
				krakenFail = true
			}
		}

		if esStr && !krakenFail {
			err = readElasticSearch("kraken", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kraken exchange function")
				krakenFail = true
			}
		}

		if influxStr && !krakenFail {
			err = readInfluxDB("kraken", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kraken exchange function")
				krakenFail = true
			}
		}

		if natsStr && !krakenFail {
			err = readNATS("kraken", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kraken exchange function")
				krakenFail = true
			}
		}

		if clickHouseStr && !krakenFail {
			err = readClickHouse("kraken", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kraken exchange function")
				krakenFail = true
			}
		}

		if !krakenFail {
			err = verifyData("kraken", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : kraken exchange function")
				krakenFail = true
			} else {
				t.Log("SUCCESS : kraken exchange function")
			}
		}
	}

	// Binance US exchange.
	var binanceUSFail bool

	if enabledExchanges["binance-us"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["binance-us"]
		s3Trades = s3AllTrades["binance-us"]

		if terStr {
			err = readTerminal("binance-us", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-us exchange function")
				binanceUSFail = true
			}
		}

		if sqlStr && !binanceUSFail {
			err = readMySQL("binance-us", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-us exchange function")
				binanceUSFail = true
			}
		}

		if esStr && !binanceUSFail {
			err = readElasticSearch("binance-us", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-us exchange function")
				binanceUSFail = true
			}
		}

		if influxStr && !binanceUSFail {
			err = readInfluxDB("binance-us", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-us exchange function")
				binanceUSFail = true
			}
		}

		if natsStr && !binanceUSFail {
			err = readNATS("binance-us", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-us exchange function")
				binanceUSFail = true
			}
		}

		if clickHouseStr && !binanceUSFail {
			err = readClickHouse("binance-us", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-us exchange function")
				binanceUSFail = true
			}
		}

		if !binanceUSFail {
			err = verifyData("binance-us", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-us exchange function")
				binanceUSFail = true
			} else {
				t.Log("SUCCESS : binance-us exchange function")
			}
		}
	}

	// OKEx exchange.
	var okexFail bool

	if enabledExchanges["okex"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["okex"]
		s3Trades = s3AllTrades["okex"]

		if terStr {
			err = readTerminal("okex", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : okex exchange function")
				okexFail = true
			}
		}

		if sqlStr && !okexFail {
			err = readMySQL("okex", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : okex exchange function")
				okexFail = true
			}
		}

		if esStr && !okexFail {
			err = readElasticSearch("okex", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : okex exchange function")
				okexFail = true
			}
		}

		if influxStr && !okexFail {
			err = readInfluxDB("okex", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : okex exchange function")
				okexFail = true
			}
		}

		if natsStr && !okexFail {
			err = readNATS("okex", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : okex exchange function")
				okexFail = true
			}
		}

		if clickHouseStr && !okexFail {
			err = readClickHouse("okex", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : okex exchange function")
				okexFail = true
			}
		}

		if !okexFail {
			err = verifyData("okex", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : okex exchange function")
				okexFail = true
			} else {
				t.Log("SUCCESS : okex exchange function")
			}
		}
	}

	// FTX US exchange.
	var ftxUSFail bool

	if enabledExchanges["ftx-us"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["ftx-us"]
		s3Trades = s3AllTrades["ftx-us"]

		if terStr {
			err = readTerminal("ftx-us", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx-us exchange function")
				ftxUSFail = true
			}
		}

		if sqlStr && !ftxUSFail {
			err = readMySQL("ftx-us", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx-us exchange function")
				ftxUSFail = true
			}
		}

		if esStr && !ftxUSFail {
			err = readElasticSearch("ftx-us", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx-us exchange function")
				ftxUSFail = true
			}
		}

		if influxStr && !ftxUSFail {
			err = readInfluxDB("ftx-us", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx-us exchange function")
				ftxUSFail = true
			}
		}

		if natsStr && !ftxUSFail {
			err = readNATS("ftx-us", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx-us exchange function")
				ftxUSFail = true
			}
		}

		if clickHouseStr && !ftxUSFail {
			err = readClickHouse("ftx-us", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx-us exchange function")
				ftxUSFail = true
			}
		}

		if !ftxUSFail {
			err = verifyData("ftx-us", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : ftx-us exchange function")
				ftxUSFail = true
			} else {
				t.Log("SUCCESS : ftx-us exchange function")
			}
		}
	}

	// HitBTC exchange.
	var hitBTCFail bool

	if enabledExchanges["hitbtc"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["hitbtc"]
		s3Trades = s3AllTrades["hitbtc"]

		if terStr {
			err = readTerminal("hitbtc", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : hitbtc exchange function")
				hitBTCFail = true
			}
		}

		if sqlStr && !hitBTCFail {
			err = readMySQL("hitbtc", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : hitbtc exchange function")
				hitBTCFail = true
			}
		}

		if esStr && !hitBTCFail {
			err = readElasticSearch("hitbtc", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : hitbtc exchange function")
				hitBTCFail = true
			}
		}

		if influxStr && !hitBTCFail {
			err = readInfluxDB("hitbtc", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : hitbtc exchange function")
				hitBTCFail = true
			}
		}

		if natsStr && !hitBTCFail {
			err = readNATS("hitbtc", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : hitbtc exchange function")
				hitBTCFail = true
			}
		}

		if clickHouseStr && !hitBTCFail {
			err = readClickHouse("hitbtc", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : hitbtc exchange function")
				hitBTCFail = true
			}
		}

		if !hitBTCFail {
			err = verifyData("hitbtc", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : hitbtc exchange function")
				hitBTCFail = true
			} else {
				t.Log("SUCCESS : hitbtc exchange function")
			}
		}
	}

	// AAX exchange.
	var aaxFail bool

	if enabledExchanges["aax"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["aax"]
		s3Trades = s3AllTrades["aax"]

		if terStr {
			err = readTerminal("aax", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : aax exchange function")
				aaxFail = true
			}
		}

		if sqlStr && !aaxFail {
			err = readMySQL("aax", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : aax exchange function")
				aaxFail = true
			}
		}

		if esStr && !aaxFail {
			err = readElasticSearch("aax", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : aax exchange function")
				aaxFail = true
			}
		}

		if influxStr && !aaxFail {
			err = readInfluxDB("aax", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : aax exchange function")
				aaxFail = true
			}
		}

		if natsStr && !aaxFail {
			err = readNATS("aax", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : aax exchange function")
				aaxFail = true
			}
		}

		if clickHouseStr && !aaxFail {
			err = readClickHouse("aax", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : aax exchange function")
				aaxFail = true
			}
		}

		if !aaxFail {
			err = verifyData("aax", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : aax exchange function")
				aaxFail = true
			} else {
				t.Log("SUCCESS : aax exchange function")
			}
		}
	}

	// Bitrue exchange.
	var bitrueFail bool

	if enabledExchanges["bitrue"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["bitrue"]
		s3Trades = s3AllTrades["bitrue"]

		if terStr {
			err = readTerminal("bitrue", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitrue exchange function")
				bitrueFail = true
			}
		}

		if sqlStr && !bitrueFail {
			err = readMySQL("bitrue", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitrue exchange function")
				bitrueFail = true
			}
		}

		if esStr && !bitrueFail {
			err = readElasticSearch("bitrue", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitrue exchange function")
				bitrueFail = true
			}
		}

		if influxStr && !bitrueFail {
			err = readInfluxDB("bitrue", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitrue exchange function")
				bitrueFail = true
			}
		}

		if natsStr && !bitrueFail {
			err = readNATS("bitrue", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitrue exchange function")
				bitrueFail = true
			}
		}

		if clickHouseStr && !bitrueFail {
			err = readClickHouse("bitrue", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitrue exchange function")
				bitrueFail = true
			}
		}

		if !bitrueFail {
			err = verifyData("bitrue", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bitrue exchange function")
				bitrueFail = true
			} else {
				t.Log("SUCCESS : bitrue exchange function")
			}
		}
	}

	// BTSE exchange.
	var btseFail bool

	if enabledExchanges["btse"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["btse"]
		s3Trades = s3AllTrades["btse"]

		if terStr {
			err = readTerminal("btse", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : btse exchange function")
				btseFail = true
			}
		}

		if sqlStr && !btseFail {
			err = readMySQL("btse", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : btse exchange function")
				btseFail = true
			}
		}

		if esStr && !btseFail {
			err = readElasticSearch("btse", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : btse exchange function")
				btseFail = true
			}
		}

		if influxStr && !btseFail {
			err = readInfluxDB("btse", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : btse exchange function")
				btseFail = true
			}
		}

		if natsStr && !btseFail {
			err = readNATS("btse", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : btse exchange function")
				btseFail = true
			}
		}

		if clickHouseStr && !btseFail {
			err = readClickHouse("btse", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : btse exchange function")
				btseFail = true
			}
		}

		if !btseFail {
			err = verifyData("btse", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : btse exchange function")
				btseFail = true
			} else {
				t.Log("SUCCESS : btse exchange function")
			}
		}
	}

	// Mexo exchange.
	var mexoFail bool

	if enabledExchanges["mexo"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["mexo"]
		s3Trades = s3AllTrades["mexo"]

		if terStr {
			err = readTerminal("mexo", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : mexo exchange function")
				mexoFail = true
			}
		}

		if sqlStr && !mexoFail {
			err = readMySQL("mexo", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : mexo exchange function")
				mexoFail = true
			}
		}

		if esStr && !mexoFail {
			err = readElasticSearch("mexo", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : mexo exchange function")
				mexoFail = true
			}
		}

		if influxStr && !mexoFail {
			err = readInfluxDB("mexo", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : mexo exchange function")
				mexoFail = true
			}
		}

		if natsStr && !mexoFail {
			err = readNATS("mexo", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : mexo exchange function")
				mexoFail = true
			}
		}

		if clickHouseStr && !mexoFail {
			err = readClickHouse("mexo", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : mexo exchange function")
				mexoFail = true
			}
		}

		if !mexoFail {
			err = verifyData("mexo", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : mexo exchange function")
				mexoFail = true
			} else {
				t.Log("SUCCESS : mexo exchange function")
			}
		}
	}

	// Bequant exchange.
	var bequantFail bool

	if enabledExchanges["bequant"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["bequant"]
		s3Trades = s3AllTrades["bequant"]

		if terStr {
			err = readTerminal("bequant", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bequant exchange function")
				bequantFail = true
			}
		}

		if sqlStr && !bequantFail {
			err = readMySQL("bequant", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bequant exchange function")
				bequantFail = true
			}
		}

		if esStr && !bequantFail {
			err = readElasticSearch("bequant", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bequant exchange function")
				bequantFail = true
			}
		}

		if influxStr && !bequantFail {
			err = readInfluxDB("bequant", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bequant exchange function")
				bequantFail = true
			}
		}

		if natsStr && !bequantFail {
			err = readNATS("bequant", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bequant exchange function")
				bequantFail = true
			}
		}

		if clickHouseStr && !bequantFail {
			err = readClickHouse("bequant", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bequant exchange function")
				bequantFail = true
			}
		}

		if !bequantFail {
			err = verifyData("bequant", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : bequant exchange function")
				bequantFail = true
			} else {
				t.Log("SUCCESS : bequant exchange function")
			}
		}
	}

	// LBank exchange.
	var lbankFail bool

	if enabledExchanges["lbank"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["lbank"]
		s3Trades = s3AllTrades["lbank"]

		if terStr {
			err = readTerminal("lbank", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : lbank exchange function")
				lbankFail = true
			}
		}

		if sqlStr && !lbankFail {
			err = readMySQL("lbank", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : lbank exchange function")
				lbankFail = true
			}
		}

		if esStr && !lbankFail {
			err = readElasticSearch("lbank", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : lbank exchange function")
				lbankFail = true
			}
		}

		if influxStr && !lbankFail {
			err = readInfluxDB("lbank", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : lbank exchange function")
				lbankFail = true
			}
		}

		if natsStr && !lbankFail {
			err = readNATS("lbank", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : lbank exchange function")
				lbankFail = true
			}
		}

		if clickHouseStr && !lbankFail {
			err = readClickHouse("lbank", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : lbank exchange function")
				lbankFail = true
			}
		}

		if !lbankFail {
			err = verifyData("lbank", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : lbank exchange function")
				lbankFail = true
			} else {
				t.Log("SUCCESS : lbank exchange function")
			}
		}
	}

	// CoinFlex exchange.
	var coinFlexFail bool

	if enabledExchanges["coinflex"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["coinflex"]
		s3Trades = s3AllTrades["coinflex"]

		if terStr {
			err = readTerminal("coinflex", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinflex exchange function")
				coinFlexFail = true
			}
		}

		if sqlStr && !coinFlexFail {
			err = readMySQL("coinflex", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinflex exchange function")
				coinFlexFail = true
			}
		}

		if esStr && !coinFlexFail {
			err = readElasticSearch("coinflex", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinflex exchange function")
				coinFlexFail = true
			}
		}

		if influxStr && !coinFlexFail {
			err = readInfluxDB("coinflex", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinflex exchange function")
				coinFlexFail = true
			}
		}

		if natsStr && !coinFlexFail {
			err = readNATS("coinflex", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinflex exchange function")
				coinFlexFail = true
			}
		}

		if clickHouseStr && !coinFlexFail {
			err = readClickHouse("coinflex", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinflex exchange function")
				coinFlexFail = true
			}
		}

		if !coinFlexFail {
			err = verifyData("coinflex", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : coinflex exchange function")
				coinFlexFail = true
			} else {
				t.Log("SUCCESS : coinflex exchange function")
			}
		}
	}

	// Binance TR exchange.
	var binanceTRFail bool

	if enabledExchanges["binance-tr"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["binance-tr"]
		s3Trades = s3AllTrades["binance-tr"]

		if terStr {
			err = readTerminal("binance-tr", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-tr exchange function")
				binanceTRFail = true
			}
		}

		if sqlStr && !binanceTRFail {
			err = readMySQL("binance-tr", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-tr exchange function")
				binanceTRFail = true
			}
		}

		if esStr && !binanceTRFail {
			err = readElasticSearch("binance-tr", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-tr exchange function")
				binanceTRFail = true
			}
		}

		if influxStr && !binanceTRFail {
			err = readInfluxDB("binance-tr", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-tr exchange function")
				binanceTRFail = true
			}
		}

		if natsStr && !binanceTRFail {
			err = readNATS("binance-tr", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-tr exchange function")
				binanceTRFail = true
			}
		}

		if clickHouseStr && !binanceTRFail {
			err = readClickHouse("binance-tr", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-tr exchange function")
				binanceTRFail = true
			}
		}

		if !binanceTRFail {
			err = verifyData("binance-tr", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : binance-tr exchange function")
				binanceTRFail = true
			} else {
				t.Log("SUCCESS : binance-tr exchange function")
			}
		}
	}

	// Crypto.com exchange.
	var cryptodotComFail bool

	if enabledExchanges["cryptodot-com"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["cryptodot-com"]
		s3Trades = s3AllTrades["cryptodot-com"]

		if terStr {
			err = readTerminal("cryptodot-com", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : cryptodot-com exchange function")
				cryptodotComFail = true
			}
		}

		if sqlStr && !cryptodotComFail {
			err = readMySQL("cryptodot-com", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : cryptodot-com exchange function")
				cryptodotComFail = true
			}
		}

		if esStr && !cryptodotComFail {
			err = readElasticSearch("cryptodot-com", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : cryptodot-com exchange function")
				cryptodotComFail = true
			}
		}

		if influxStr && !cryptodotComFail {
			err = readInfluxDB("cryptodot-com", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : cryptodot-com exchange function")
				cryptodotComFail = true
			}
		}

		if natsStr && !cryptodotComFail {
			err = readNATS("cryptodot-com", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : cryptodot-com exchange function")
				cryptodotComFail = true
			}
		}

		if clickHouseStr && !cryptodotComFail {
			err = readClickHouse("cryptodot-com", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : cryptodot-com exchange function")
				cryptodotComFail = true
			}
		}

		if !cryptodotComFail {
			err = verifyData("cryptodot-com", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : cryptodot-com exchange function")
				cryptodotComFail = true
			} else {
				t.Log("SUCCESS : cryptodot-com exchange function")
			}
		}
	}

	// FMFW.io exchange.
	var fmfwioFail bool

	if enabledExchanges["fmfwio"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["fmfwio"]
		s3Trades = s3AllTrades["fmfwio"]

		if terStr {
			err = readTerminal("fmfwio", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : fmfwio exchange function")
				fmfwioFail = true
			}
		}

		if sqlStr && !fmfwioFail {
			err = readMySQL("fmfwio", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : fmfwio exchange function")
				fmfwioFail = true
			}
		}

		if esStr && !fmfwioFail {
			err = readElasticSearch("fmfwio", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : fmfwio exchange function")
				fmfwioFail = true
			}
		}

		if influxStr && !fmfwioFail {
			err = readInfluxDB("fmfwio", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : fmfwio exchange function")
				fmfwioFail = true
			}
		}

		if natsStr && !fmfwioFail {
			err = readNATS("fmfwio", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : fmfwio exchange function")
				fmfwioFail = true
			}
		}

		if clickHouseStr && !fmfwioFail {
			err = readClickHouse("fmfwio", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : fmfwio exchange function")
				fmfwioFail = true
			}
		}

		if !fmfwioFail {
			err = verifyData("fmfwio", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : fmfwio exchange function")
				fmfwioFail = true
			} else {
				t.Log("SUCCESS : fmfwio exchange function")
			}
		}
	}

	// Changelly Pro exchange.
	var changellyProFail bool

	if enabledExchanges["changelly-pro"] {
		terTickers = make(map[string]storage.Ticker)
		terTrades = make(map[string]storage.Trade)
		mysqlTickers = make(map[string]storage.Ticker)
		mysqlTrades = make(map[string]storage.Trade)
		esTickers = make(map[string]storage.Ticker)
		esTrades = make(map[string]storage.Trade)
		influxTickers = make(map[string]storage.Ticker)
		influxTrades = make(map[string]storage.Trade)
		natsTickers = make(map[string]storage.Ticker)
		natsTrades = make(map[string]storage.Trade)
		clickHouseTickers = make(map[string]storage.Ticker)
		clickHouseTrades = make(map[string]storage.Trade)
		s3Tickers = s3AllTickers["changelly-pro"]
		s3Trades = s3AllTrades["changelly-pro"]

		if terStr {
			err = readTerminal("changelly-pro", terTickers, terTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : changelly-pro exchange function")
				changellyProFail = true
			}
		}

		if sqlStr && !changellyProFail {
			err = readMySQL("changelly-pro", mysqlTickers, mysqlTrades, mysql)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : changelly-pro exchange function")
				changellyProFail = true
			}
		}

		if esStr && !changellyProFail {
			err = readElasticSearch("changelly-pro", esTickers, esTrades, es)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : changelly-pro exchange function")
				changellyProFail = true
			}
		}

		if influxStr && !changellyProFail {
			err = readInfluxDB("changelly-pro", influxTickers, influxTrades, influx)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : changelly-pro exchange function")
				changellyProFail = true
			}
		}

		if natsStr && !changellyProFail {
			err = readNATS("changelly-pro", natsTickers, natsTrades)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : changelly-pro exchange function")
				changellyProFail = true
			}
		}

		if clickHouseStr && !changellyProFail {
			err = readClickHouse("changelly-pro", clickHouseTickers, clickHouseTrades, clickhouse)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : changelly-pro exchange function")
				changellyProFail = true
			}
		}

		if !changellyProFail {
			err = verifyData("changelly-pro", terTickers, terTrades, mysqlTickers, mysqlTrades, esTickers, esTrades, influxTickers, influxTrades, natsTickers, natsTrades, clickHouseTickers, clickHouseTrades, s3Tickers, s3Trades, &cfg)
			if err != nil {
				t.Log("ERROR : " + err.Error())
				t.Error("FAILURE : changelly-pro exchange function")
				changellyProFail = true
			} else {
				t.Log("SUCCESS : changelly-pro exchange function")
			}
		}
	}

	if ftxFail || coinbaseProFail || binanceFail || bitfinexFail || huobiFail || gateioFail || kucoinFail || bitstampFail || bybitFail || probitFail || geminiFail || bitmartFail || digifinexFail || ascendexFail || krakenFail || binanceUSFail || okexFail || ftxUSFail || hitBTCFail || aaxFail || bitrueFail || btseFail || mexoFail || bequantFail || lbankFail || coinFlexFail || binanceTRFail || cryptodotComFail || fmfwioFail || changellyProFail {
		t.Log("INFO : May be 2 minute app execution time is not good enough to get the data. Try to increase it before actual debugging.")
	}
}

// readTerminal reads ticker and trade data for an exchange from a file, which has been set as terminal output,
// into passed in maps.
func readTerminal(exchName string, terTickers map[string]storage.Ticker, terTrades map[string]storage.Trade) error {
	outFile, err := os.OpenFile("./data_test/ter_storage_test.txt", os.O_RDONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer outFile.Close()
	rd := bufio.NewReader(outFile)
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if line == "\n" {
			continue
		}
		words := strings.Fields(line)
		if words[1] == exchName {
			switch words[0] {
			case "Ticker":
				key := words[2]
				price, err := strconv.ParseFloat(words[3], 64)
				if err != nil {
					return err
				}
				val := storage.Ticker{
					Price: price,
				}
				terTickers[key] = val
			case "Trade":
				key := words[2]
				size, err := strconv.ParseFloat(words[3], 64)
				if err != nil {
					return err
				}
				price, err := strconv.ParseFloat(words[4], 64)
				if err != nil {
					return err
				}
				val := storage.Trade{
					Size:  size,
					Price: price,
				}
				terTrades[key] = val
			}
		}
	}
	return nil
}

// readMySQL reads ticker and trade data for an exchange from mysql into passed in maps.
func readMySQL(exchName string, mysqlTickers map[string]storage.Ticker, mysqlTrades map[string]storage.Trade, mysql *storage.MySQL) error {
	var ctx context.Context
	if mysql.Cfg.ReqTimeoutSec > 0 {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(mysql.Cfg.ReqTimeoutSec)*time.Second)
		ctx = timeoutCtx
		defer cancel()
	} else {
		ctx = context.Background()
	}
	tickerRows, err := mysql.DB.QueryContext(ctx, "select market, avg(price) as price from ticker where exchange = ? group by market", exchName)
	if err != nil {
		return err
	}
	defer tickerRows.Close()
	for tickerRows.Next() {
		var market string
		var price float64
		err = tickerRows.Scan(&market, &price)
		if err != nil {
			return err
		}
		val := storage.Ticker{
			Price: price,
		}
		mysqlTickers[market] = val
	}
	err = tickerRows.Err()
	if err != nil {
		return err
	}

	if mysql.Cfg.ReqTimeoutSec > 0 {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(mysql.Cfg.ReqTimeoutSec)*time.Second)
		ctx = timeoutCtx
		defer cancel()
	} else {
		ctx = context.Background()
	}
	tradeRows, err := mysql.DB.QueryContext(ctx, "select market, avg(size) as size, avg(price) as price from trade where exchange = ? group by market", exchName)
	if err != nil {
		return err
	}
	defer tradeRows.Close()
	for tradeRows.Next() {
		var market string
		var size float64
		var price float64
		err = tradeRows.Scan(&market, &size, &price)
		if err != nil {
			return err
		}
		val := storage.Trade{
			Size:  size,
			Price: price,
		}
		mysqlTrades[market] = val
	}
	err = tradeRows.Err()
	if err != nil {
		return err
	}
	return nil
}

// readElasticSearch reads ticker and trade data for an exchange from elastic search into passed in maps.
func readElasticSearch(exchName string, esTickers map[string]storage.Ticker, esTrades map[string]storage.Trade, es *storage.ElasticSearch) error {
	var buf bytes.Buffer
	query := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"match": map[string]interface{}{
				"exchange": exchName,
			},
		},
		"aggs": map[string]interface{}{
			"by_channel": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "channel",
				},
				"aggs": map[string]interface{}{
					"by_market": map[string]interface{}{
						"terms": map[string]interface{}{
							"field": "market",
						},
						"aggs": map[string]interface{}{
							"size": map[string]interface{}{
								"avg": map[string]interface{}{
									"field": "size",
								},
							},
							"price": map[string]interface{}{
								"avg": map[string]interface{}{
									"field": "price",
								},
							},
						},
					},
				},
			},
		},
	}
	if err := jsoniter.NewEncoder(&buf).Encode(query); err != nil {
		return err
	}

	var ctx context.Context
	if es.Cfg.ReqTimeoutSec > 0 {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(es.Cfg.ReqTimeoutSec)*time.Second)
		ctx = timeoutCtx
		defer cancel()
	} else {
		ctx = context.Background()
	}
	res, err := es.ES.Search(
		es.ES.Search.WithIndex(es.IndexName),
		es.ES.Search.WithBody(&buf),
		es.ES.Search.WithTrackTotalHits(false),
		es.ES.Search.WithPretty(),
		es.ES.Search.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return err
	}

	var esResp esResp
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return err
	}
	for _, channel := range esResp.Aggregations.ByChannel.Buckets {
		switch channel.Key {
		case "ticker":
			for _, market := range channel.ByMarket.Buckets {
				val := storage.Ticker{
					Price: market.Price.Value,
				}
				esTickers[market.Key] = val
			}
		case "trade":
			for _, market := range channel.ByMarket.Buckets {
				val := storage.Trade{
					Size:  market.Size.Value,
					Price: market.Price.Value,
				}
				esTrades[market.Key] = val
			}
		}
	}
	return nil
}

// readInfluxDB reads ticker and trade data for an exchange from influxdb into passed in maps.
func readInfluxDB(exchName string, influxTickers map[string]storage.Ticker, influxTrades map[string]storage.Trade, influx *storage.InfluxDB) error {
	var ctx context.Context
	if influx.Cfg.ReqTimeoutSec > 0 {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(influx.Cfg.ReqTimeoutSec)*time.Second)
		ctx = timeoutCtx
		defer cancel()
	} else {
		ctx = context.Background()
	}
	result, err := influx.QuerryAPI.Query(ctx, `
	                         from(bucket: "`+influx.Cfg.Bucket+`")
                             |> range(start: -1h)
                             |> filter(fn: (r) =>
                                  r._measurement == "ticker" and
                                  r._field == "price" and
                                  r.exchange == "`+exchName+`" 
                                )
                             |> group(columns: ["market"])
                             |> mean()
                        `)
	if err != nil {
		return err
	}
	for result.Next() {
		market, ok := result.Record().ValueByKey("market").(string)
		if !ok {
			return errors.New("cannot convert influxdb trade market to string")
		}
		price, ok := result.Record().Value().(float64)
		if !ok {
			return errors.New("cannot convert influxdb trade price to float")
		}
		val := storage.Ticker{
			Price: price,
		}
		influxTickers[market] = val
	}
	err = result.Err()
	if err != nil {
		return err
	}

	if influx.Cfg.ReqTimeoutSec > 0 {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(influx.Cfg.ReqTimeoutSec)*time.Second)
		ctx = timeoutCtx
		defer cancel()
	} else {
		ctx = context.Background()
	}
	result, err = influx.QuerryAPI.Query(ctx, `
	                         from(bucket: "`+influx.Cfg.Bucket+`")
                             |> range(start: -1h)
                             |> filter(fn: (r) =>
                                  r._measurement == "trade" and
                                  (r._field == "size" or r._field == "price") and
                                  r.exchange == "`+exchName+`" 
                                )
                             |> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
                             |> group(columns: ["market"])
                             |> reduce(
                                  identity: {
                                      count:  1.0,
                                      size_sum: 0.0,
                                      size_avg: 0.0,
                                      price_sum: 0.0,
                                      price_avg: 0.0
                                  },
                                  fn: (r, accumulator) => ({
                                      count:  accumulator.count + 1.0,
                                      size_sum: accumulator.size_sum + r.size,
                                      size_avg: accumulator.size_sum / accumulator.count,
                                      price_sum: accumulator.price_sum + r.price,
                                      price_avg: accumulator.price_sum / accumulator.count
                                  })
                                )
                             |> drop(columns: ["count", "size_sum", "price_sum"])
	                    `)
	if err != nil {
		return err
	}
	for result.Next() {
		market, ok := result.Record().ValueByKey("market").(string)
		if !ok {
			return errors.New("cannot convert influxdb trade market to string")
		}
		size, ok := result.Record().ValueByKey("size_avg").(float64)
		if !ok {
			return errors.New("cannot convert influxdb trade size to float")
		}
		price, ok := result.Record().ValueByKey("price_avg").(float64)
		if !ok {
			return errors.New("cannot convert influxdb trade price to float")
		}
		val := storage.Trade{
			Size:  size,
			Price: price,
		}
		influxTrades[market] = val
	}
	err = result.Err()
	if err != nil {
		return err
	}
	return nil
}

// readNATS reads ticker and trade data for an exchange from a file, which has been set as nats output,
// into passed in maps.
func readNATS(exchName string, natsTickers map[string]storage.Ticker, natsTrades map[string]storage.Trade) error {
	outFile, err := os.OpenFile("./data_test/nats_storage_test.txt", os.O_RDONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer outFile.Close()
	rd := bufio.NewReader(outFile)
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if line == "\n" {
			continue
		}
		words := strings.Fields(line)
		if words[1] == exchName {
			switch words[0] {
			case "Ticker":
				key := words[2]
				price, err := strconv.ParseFloat(words[3], 64)
				if err != nil {
					return err
				}
				val := storage.Ticker{
					Price: price,
				}
				natsTickers[key] = val
			case "Trade":
				key := words[2]
				size, err := strconv.ParseFloat(words[3], 64)
				if err != nil {
					return err
				}
				price, err := strconv.ParseFloat(words[4], 64)
				if err != nil {
					return err
				}
				val := storage.Trade{
					Size:  size,
					Price: price,
				}
				natsTrades[key] = val
			}
		}
	}
	return nil
}

// readClickHouse reads ticker and trade data for an exchange from clickhouse into passed in maps.
func readClickHouse(exchName string, clickHouseTickers map[string]storage.Ticker, clickHouseTrades map[string]storage.Trade, clickHouse *storage.ClickHouse) error {
	tickerRows, err := clickHouse.DB.Query("select market, avg(price) as price from ticker where exchange = ? group by market", exchName)
	if err != nil {
		return err
	}
	defer tickerRows.Close()
	for tickerRows.Next() {
		var market string
		var price float64
		err = tickerRows.Scan(&market, &price)
		if err != nil {
			return err
		}
		val := storage.Ticker{
			Price: price,
		}
		clickHouseTickers[market] = val
	}
	err = tickerRows.Err()
	if err != nil {
		return err
	}

	tradeRows, err := clickHouse.DB.Query("select market, avg(size) as size, avg(price) as price from trade where exchange = ? group by market", exchName)
	if err != nil {
		return err
	}
	defer tradeRows.Close()
	for tradeRows.Next() {
		var market string
		var size float64
		var price float64
		err = tradeRows.Scan(&market, &size, &price)
		if err != nil {
			return err
		}
		val := storage.Trade{
			Size:  size,
			Price: price,
		}
		clickHouseTrades[market] = val
	}
	err = tradeRows.Err()
	if err != nil {
		return err
	}
	return nil
}

// readS3 reads ticker and trade data for all the exchanges from s3 into passed in maps.
func readS3(s3AllTickers map[string]map[string]storage.Ticker, s3AllTrades map[string]map[string]storage.Trade, s3 *storage.S3) error {
	var lastObjKey string
	var objInput *awss3.ListObjectsV2Input
	for {
		if lastObjKey == "" {
			objInput = &awss3.ListObjectsV2Input{
				Bucket: &s3.Cfg.Bucket,
			}
		} else {
			objInput = &awss3.ListObjectsV2Input{
				Bucket:     &s3.Cfg.Bucket,
				StartAfter: &lastObjKey,
			}
		}
		objResp, err := s3.Client.ListObjectsV2(context.TODO(), objInput)
		if err != nil {
			return err
		}
		if len(objResp.Contents) > 0 {
			for _, item := range objResp.Contents {
				lastObjKey = *item.Key
				detailInput := &awss3.GetObjectInput{
					Bucket: &s3.Cfg.Bucket,
					Key:    item.Key,
				}
				detailResp, err := s3.Client.GetObject(context.TODO(), detailInput)
				if err != nil {
					return err
				}
				if strings.Contains(lastObjKey, "ticker") {
					data := []storage.Ticker{}
					if err := jsoniter.NewDecoder(detailResp.Body).Decode(&data); err != nil {
						return err
					}
					for i := range data {
						ticker := data[i]
						s3SubTickers, pres := s3AllTickers[ticker.Exchange]
						if !pres {
							s3SubTickers = make(map[string]storage.Ticker)
						}
						s3SubTickers[ticker.MktCommitName] = ticker
						s3AllTickers[ticker.Exchange] = s3SubTickers
					}
				} else {
					data := []storage.Trade{}
					if err := jsoniter.NewDecoder(detailResp.Body).Decode(&data); err != nil {
						return err
					}
					for i := range data {
						trade := data[i]
						s3SubTrades, pres := s3AllTrades[trade.Exchange]
						if !pres {
							s3SubTrades = make(map[string]storage.Trade)
						}
						s3SubTrades[trade.MktCommitName] = trade
						s3AllTrades[trade.Exchange] = s3SubTrades
					}
				}
				detailResp.Body.Close()
			}
		} else {
			break
		}
	}
	return nil
}

// verifyData checks whether all the configured storage system for an exchange got the required data or not.
func verifyData(exchName string, terTickers map[string]storage.Ticker, terTrades map[string]storage.Trade,
	mysqlTickers map[string]storage.Ticker, mysqlTrades map[string]storage.Trade,
	esTickers map[string]storage.Ticker, esTrades map[string]storage.Trade,
	influxTickers map[string]storage.Ticker, influxTrades map[string]storage.Trade,
	natsTickers map[string]storage.Ticker, natsTrades map[string]storage.Trade,
	clickHouseTickers map[string]storage.Ticker, clickHouseTrades map[string]storage.Trade,
	s3Tickers map[string]storage.Ticker, s3Trades map[string]storage.Trade,
	cfg *config.Config) error {

	for _, exch := range cfg.Exchanges {
		if exch.Name == exchName {
			for _, market := range exch.Markets {
				var marketCommitName string
				if market.CommitName != "" {
					marketCommitName = market.CommitName
				} else {
					marketCommitName = market.ID
				}
				for _, info := range market.Info {
					switch info.Channel {
					case "ticker":
						for _, str := range info.Storages {
							switch str {
							case "terminal":
								terTicker := terTickers[marketCommitName]
								if terTicker.Price <= 0 {
									return fmt.Errorf("%s ticker data stored in terminal is not complete", market.ID)
								}
							case "mysql":
								sqlTicker := mysqlTickers[marketCommitName]
								if sqlTicker.Price <= 0 {
									return fmt.Errorf("%s ticker data stored in mysql is not complete", market.ID)
								}
							case "elastic_search":
								esTicker := esTickers[marketCommitName]
								if esTicker.Price <= 0 {
									return fmt.Errorf("%s ticker data stored in elastic search is not complete", market.ID)
								}
							case "influxdb":
								influxTicker := influxTickers[marketCommitName]
								if influxTicker.Price <= 0 {
									return fmt.Errorf("%s ticker data stored in influxdb is not complete", market.ID)
								}
							case "nats":
								natsTicker := natsTickers[marketCommitName]
								if natsTicker.Price <= 0 {
									return fmt.Errorf("%s ticker data stored in nats is not complete", market.ID)
								}
							case "clickhouse":
								clickHouseTicker := clickHouseTickers[marketCommitName]
								if clickHouseTicker.Price <= 0 {
									return fmt.Errorf("%s ticker data stored in clickhouse is not complete", market.ID)
								}
							case "s3":
								s3Ticker := s3Tickers[marketCommitName]
								if s3Ticker.Price <= 0 {
									return fmt.Errorf("%s ticker data stored in s3 is not complete", market.ID)
								}
							}
						}
					case "trade":
						for _, str := range info.Storages {
							switch str {
							case "terminal":
								terTrade := terTrades[marketCommitName]
								if terTrade.Size <= 0 || terTrade.Price <= 0 {
									return fmt.Errorf("%s trade data stored in terminal is not complete", market.ID)
								}
							case "mysql":
								sqlTrade := mysqlTrades[marketCommitName]
								if sqlTrade.Size <= 0 || sqlTrade.Price <= 0 {
									return fmt.Errorf("%s trade data stored in mysql is not complete", market.ID)
								}
							case "elastic_search":
								esTrade := esTrades[marketCommitName]
								if esTrade.Size <= 0 || esTrade.Price <= 0 {
									return fmt.Errorf("%s trade data stored in elastic search is not complete", market.ID)
								}
							case "influxdb":
								influxTrade := influxTrades[marketCommitName]
								if influxTrade.Size <= 0 || influxTrade.Price <= 0 {
									return fmt.Errorf("%s trade data stored in influxdb is not complete", market.ID)
								}
							case "nats":
								natsTrade := natsTrades[marketCommitName]
								if natsTrade.Size <= 0 || natsTrade.Price <= 0 {
									return fmt.Errorf("%s trade data stored in nats is not complete", market.ID)
								}
							case "clickhouse":
								clickHouseTrade := clickHouseTrades[marketCommitName]
								if clickHouseTrade.Size <= 0 || clickHouseTrade.Price <= 0 {
									return fmt.Errorf("%s trade data stored in clickhouse is not complete", market.ID)
								}
							case "s3":
								s3Trade := s3Trades[marketCommitName]
								if s3Trade.Size <= 0 || s3Trade.Price <= 0 {
									return fmt.Errorf("%s trade data stored in s3 is not complete", market.ID)
								}
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// Subscribe to NATS subject. Write received data to a file.
func natsSub(subject string, out io.Writer, nats *storage.NATS, t *testing.T) {
	if _, err := nats.Basic.Subscribe(subject, func(m *nc.Msg) {
		if strings.HasSuffix(m.Subject, "ticker") {
			ticker := storage.Ticker{}
			err := jsoniter.Unmarshal(m.Data, &ticker)
			if err != nil {
				t.Log("ERROR : " + err.Error())
			}
			fmt.Fprintf(out, "%-15s%-15s%-15s%20f\n\n", "Ticker", ticker.Exchange, ticker.MktCommitName, ticker.Price)
		} else {
			trade := storage.Trade{}
			err := jsoniter.Unmarshal(m.Data, &trade)
			if err != nil {
				t.Log("ERROR : " + err.Error())
			}
			fmt.Fprintf(out, "%-15s%-15s%-5s%20f%20f\n\n", "Trade", trade.Exchange, trade.MktCommitName, trade.Size, trade.Price)
		}
	}); err != nil {
		t.Log("ERROR : " + err.Error())
	}
}

type esResp struct {
	Aggregations struct {
		ByChannel struct {
			Buckets []struct {
				Key      string `json:"key"`
				ByMarket struct {
					Buckets []struct {
						Key  string `json:"key"`
						Size struct {
							Value float64 `json:"value"`
						} `json:"size"`
						Price struct {
							Value float64 `json:"value"`
						} `json:"price"`
					} `json:"buckets"`
				} `json:"by_market"`
			} `json:"buckets"`
		} `json:"by_channel"`
	} `json:"aggregations"`
}
