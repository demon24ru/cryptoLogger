package config

const (
	// FtxWebsocketURL is the ftx exchange websocket url.
	FtxWebsocketURL = "wss://ftx.com/ws/"
	// FtxRESTBaseURL is the ftx exchange base REST url.
	FtxRESTBaseURL = "https://ftx.com/api/"

	// CoinbaseProWebsocketURL is the coinbase-pro exchange websocket url.
	CoinbaseProWebsocketURL = "wss://ws-feed.pro.coinbase.com/"
	// CoinbaseProRESTBaseURL is the coinbase-pro exchange base REST url.
	CoinbaseProRESTBaseURL = "https://api.pro.coinbase.com/"

	// BinanceWebsocketURL is the binance exchange websocket url.
	BinanceWebsocketURL = "wss://stream.binance.com:9443/ws"
	// BinanceRESTBaseURL is the binance exchange base REST url.
	BinanceRESTBaseURL = "https://api.binance.com/api/v3/"

	// BitfinexWebsocketURL is the bitfinex exchange websocket url.
	BitfinexWebsocketURL = "wss://api-pub.bitfinex.com/ws/2"
	// BitfinexRESTBaseURL is the bitfinex exchange base REST url.
	BitfinexRESTBaseURL = "https://api-pub.bitfinex.com/v2/"

	// HuobiWebsocketURL is the huobi exchange websocket url.
	HuobiWebsocketURL = "wss://api.huobi.pro/ws"
	// HuobiRESTBaseURL is the huobi exchange base REST url.
	HuobiRESTBaseURL = "https://api.huobi.pro/"

	// GateioWebsocketURL is the gateio exchange websocket url.
	GateioWebsocketURL = "wss://api.gateio.ws/ws/v4/"
	// GateioRESTBaseURL is the gateio exchange base REST url.
	GateioRESTBaseURL = "https://api.gateio.ws/api/v4/"

	// KucoinRESTBaseURL is the kucoin exchange base REST url.
	KucoinRESTBaseURL = "https://api.kucoin.com/api/v1/"
	KucoinRESTV3URL = "https://api.kucoin.com/api/v3/"

	// BitstampWebsocketURL is the bitstamp exchange websocket url.
	BitstampWebsocketURL = "wss://ws.bitstamp.net/"
	// BitstampRESTBaseURL is the bitstamp exchange base REST url.
	BitstampRESTBaseURL = "https://www.bitstamp.net/api/v2/"

	// BybitWebsocketURL is the bybit exchange websocket url.
	BybitWebsocketURL = "wss://stream.bybit.com/realtime_public"
	// BybitRESTBaseURL is the bybit exchange base REST url.
	BybitRESTBaseURL = "https://api.bybit.com/"

	// ProbitWebsocketURL is the probit exchange websocket url.
	ProbitWebsocketURL = "wss://api.probit.com/api/exchange/v1/ws"
	// ProbitRESTBaseURL is the probit exchange base REST url.
	ProbitRESTBaseURL = "https://api.probit.com/api/exchange/v1/"

	// GeminiWebsocketURL is the gemini exchange websocket url.
	GeminiWebsocketURL = "wss://api.gemini.com/v2/marketdata"
	// GeminiRESTBaseURL is the gemini exchange base REST url.
	GeminiRESTBaseURL = "https://api.gemini.com/v1/"

	// BitmartWebsocketURL is the bitmart exchange websocket url.
	BitmartWebsocketURL = "wss://ws-manager-compress.bitmart.com?protocol=1.1"
	// BitmartRESTBaseURL is the bitmart exchange base REST url.
	BitmartRESTBaseURL = "https://api-cloud.bitmart.com/spot/v1/"

	// DigifinexWebsocketURL is the digifinex exchange websocket url.
	DigifinexWebsocketURL = "wss://openapi.digifinex.com/ws/v1/"
	// DigifinexRESTBaseURL is the digifinex exchange base REST url.
	DigifinexRESTBaseURL = "https://openapi.digifinex.com/v3/"

	// AscendexWebsocketURL is the ascendex exchange websocket url.
	AscendexWebsocketURL = "wss://ascendex.com/0/api/pro/v1/stream"
	// AscendexRESTBaseURL is the ascendex exchange base REST url.
	AscendexRESTBaseURL = "https://ascendex.com/api/pro/v1/"

	// KrakenWebsocketURL is the kraken exchange websocket url.
	KrakenWebsocketURL = "wss://ws.kraken.com"
	// KrakenRESTBaseURL is the kraken exchange base REST url.
	KrakenRESTBaseURL = "https://api.kraken.com/0/public/"

	// BinanceUSWebsocketURL is the binance-us exchange websocket url.
	BinanceUSWebsocketURL = "wss://stream.binance.us:9443/ws"
	// BinanceUSRESTBaseURL is the binance-us exchange base REST url.
	BinanceUSRESTBaseURL = "https://api.binance.us/api/v3/"

	// OKExWebsocketURL is the okex exchange websocket url.
	OKExWebsocketURL = "wss://ws.okex.com:8443/ws/v5/public"
	// OKExRESTBaseURL is the okex exchange base REST url.
	OKExRESTBaseURL = "https://www.okex.com/api/v5/"

	// FtxUSWebsocketURL is the ftx-us exchange websocket url.
	FtxUSWebsocketURL = "wss://ftx.us/ws/"
	// FtxUSRESTBaseURL is the ftx-us exchange base REST url.
	FtxUSRESTBaseURL = "https://ftx.us/api/"

	// HitBTCWebsocketURL is the hitbtc websocket url.
	HitBTCWebsocketURL = "wss://api.hitbtc.com/api/3/ws/public"
	// HitBTCRESTBaseURL is the hitbtc base REST url.
	HitBTCRESTBaseURL = "https://api.hitbtc.com/api/3/public/"

	// AAXWebsocketURL is the aax websocket url.
	AAXWebsocketURL = "wss://realtime.aax.com/marketdata/v2/"
	// AAXRESTBaseURL is the aax base REST url.
	AAXRESTBaseURL = "https://api.aax.com/v2/"

	// BitrueWebsocketURL is the bitrue exchange websocket url.
	BitrueWebsocketURL = "wss://ws.bitrue.com/kline-api/ws"
	// BitrueRESTBaseURL is the bitrue exchange base REST url.
	BitrueRESTBaseURL = "https://www.bitrue.com/api/v1/"

	// BTSEWebsocketURL is the btse exchange websocket url.
	BTSEWebsocketURL = "wss://ws.btse.com/ws/spot"
	// BTSERESTBaseURL is the btse exchange base REST url.
	BTSERESTBaseURL = "https://api.btse.com/spot/api/v3.2/"

	// MexoWebsocketURL is the mexo exchange websocket url.
	MexoWebsocketURL = "wss://wsapi.mexo.io/openapi/quote/ws/v1"
	// MexoRESTBaseURL is the mexo exchange base REST url.
	MexoRESTBaseURL = "https://api.mexo.io/openapi/"

	// BequantWebsocketURL is the bequant exchange websocket url.
	BequantWebsocketURL = "wss://api.bequant.io/api/3/ws/public"
	// BequantRESTBaseURL is the bequant exchange base REST url.
	BequantRESTBaseURL = "https://api.bequant.io/api/3/public/"

	// LBankWebsocketURL is the lbank exchange websocket url.
	LBankWebsocketURL = "wss://www.lbkex.net/ws/V2/"
	// LBankRESTBaseURL is the lbank exchange base REST url.
	LBankRESTBaseURL = "https://www.lbkex.net/v2/"

	// CoinFlexWebsocketURL is the coinflex exchange websocket url.
	CoinFlexWebsocketURL = "wss://v2api.coinflex.com/v2/websocket"
	// CoinFlexRESTBaseURL is the coinflex exchange base REST url.
	CoinFlexRESTBaseURL = "https://v2api.coinflex.com/v2/"

	// BinanceTRWebsocketURL is the binance-tr exchange websocket url.
	BinanceTRWebsocketURL = "wss://stream-cloud.trbinance.com/ws"
	// BinanceTRRESTBaseURL is the binance-tr exchange base REST url.
	BinanceTRRESTBaseURL = "https://api.binance.me/api/v3/"
	// BinanceTRRESTMktBaseURL is the binance-tr exchange base REST market url.
	BinanceTRRESTMktBaseURL = "https://www.trbinance.com/open/v1/"

	// CryptodotComWebsocketURL is the cryptodot-com exchange websocket url.
	CryptodotComWebsocketURL = "wss://stream.crypto.com/v2/market"
	// CryptodotComRESTBaseURL is the cryptodot-com exchange base REST url.
	CryptodotComRESTBaseURL = "https://api.crypto.com/v2/public/"

	// FmfwioWebsocketURL is the fmfwio exchange websocket url.
	FmfwioWebsocketURL = "wss://api.fmfw.io/api/3/ws/public"
	// FmfwioRESTBaseURL is the fmfwio exchange base REST url.
	FmfwioRESTBaseURL = "https://api.fmfw.io/api/3/public/"

	// ChangellyProWebsocketURL is the changelly-pro exchange websocket url.
	ChangellyProWebsocketURL = "wss://api.pro.changelly.com/api/3/ws/public"
	// ChangellyProRESTBaseURL is the changelly-pro exchange base REST url.
	ChangellyProRESTBaseURL = "https://api.pro.changelly.com/api/3/public/"
)

// Config contains config values for the app.
// Struct values are loaded from user defined JSON config file.
type Config struct {
	Exchanges  []Exchange `json:"exchanges"`
	Connection Connection `json:"connection"`
	Log        Log        `json:"log"`
}

// Exchange contains config values for different exchanges.
type Exchange struct {
	Name    string   `json:"name"`
	Markets []Market `json:"markets"`
	Retry   Retry    `json:"retry"`
}

// Market contains config values for different markets.
type Market struct {
	ID         string `json:"id"`
	Info       []Info `json:"info"`
	CommitName string `json:"commit_name"`
}

// Info contains config values for different market channels.
type Info struct {
	Channel          string   `json:"channel"`
	Connector        string   `json:"connector"`
	WsConsiderIntSec int      `json:"websocket_consider_interval_sec"`
	RESTPingIntSec   int      `json:"rest_ping_interval_sec"`
	Storages         []string `json:"storages"`
}

// Retry contains config values for retry process.
type Retry struct {
	Number   int `json:"number"`
	GapSec   int `json:"gap_sec"`
	ResetSec int `json:"reset_sec"`
}

// Connection contains config values for different API and storage connections.
type Connection struct {
	WS         WS         `json:"websocket"`
	REST       REST       `json:"rest"`
	Terminal   Terminal   `json:"terminal"`
	MySQL      MySQL      `json:"mysql"`
	ES         ES         `json:"elastic_search"`
	InfluxDB   InfluxDB   `json:"influxdb"`
	NATS       NATS       `json:"nats"`
	ClickHouse ClickHouse `json:"clickhouse"`
	S3         S3         `json:"s3"`
}

// WS contains config values for websocket connection.
type WS struct {
	ConnTimeoutSec int `json:"conn_timeout_sec"`
	ReadTimeoutSec int `json:"read_timeout_sec"`
}

// REST contains config values for REST API connection.
type REST struct {
	ReqTimeoutSec       int `json:"request_timeout_sec"`
	MaxIdleConns        int `json:"max_idle_conns"`
	MaxIdleConnsPerHost int `json:"max_idle_conns_per_host"`
}

// Terminal contains config values for terminal display.
type Terminal struct {
	TickerCommitBuf int `json:"ticker_commit_buffer"`
	TradeCommitBuf  int `json:"trade_commit_buffer"`
	Level2CommitBuf  int `json:"level2_commit_buffer"`
	OrdersBookCommitBuf  int `json:"orders_book_commit_buffer"`
}

// MySQL contains config values for mysql.
type MySQL struct {
	User               string `josn:"user"`
	Password           string `json:"password"`
	URL                string `json:"URL"`
	Schema             string `json:"schema"`
	ReqTimeoutSec      int    `json:"request_timeout_sec"`
	ConnMaxLifetimeSec int    `json:"conn_max_lifetime_sec"`
	MaxOpenConns       int    `json:"max_open_conns"`
	MaxIdleConns       int    `json:"max_idle_conns"`
	TickerCommitBuf    int    `json:"ticker_commit_buffer"`
	TradeCommitBuf     int    `json:"trade_commit_buffer"`
}

// ES contains config values for elastic search.
type ES struct {
	Addresses           []string `json:"addresses"`
	Username            string   `json:"username"`
	Password            string   `json:"password"`
	IndexName           string   `json:"index_name"`
	ReqTimeoutSec       int      `json:"request_timeout_sec"`
	MaxIdleConns        int      `json:"max_idle_conns"`
	MaxIdleConnsPerHost int      `json:"max_idle_conns_per_host"`
	TickerCommitBuf     int      `json:"ticker_commit_buffer"`
	TradeCommitBuf      int      `json:"trade_commit_buffer"`
}

// InfluxDB contains config values for influxdb.
type InfluxDB struct {
	Organization    string `josn:"organization"`
	Bucket          string `json:"bucket"`
	Token           string `json:"token"`
	URL             string `json:"URL"`
	ReqTimeoutSec   int    `json:"request_timeout_sec"`
	MaxIdleConns    int    `json:"max_idle_conns"`
	TickerCommitBuf int    `json:"ticker_commit_buffer"`
	TradeCommitBuf  int    `json:"trade_commit_buffer"`
}

// NATS contains config values for nats.
type NATS struct {
	Addresses       []string `json:"addresses"`
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	SubjectBaseName string   `json:"subject_base_name"`
	ReqTimeoutSec   int      `json:"request_timeout_sec"`
	TickerCommitBuf int      `json:"ticker_commit_buffer"`
	TradeCommitBuf  int      `json:"trade_commit_buffer"`
}

// ClickHouse contains config values for clickhouse.
type ClickHouse struct {
	User            string   `josn:"user"`
	Password        string   `json:"password"`
	URL             string   `json:"URL"`
	Schema          string   `json:"schema"`
	ReqTimeoutSec   int      `json:"request_timeout_sec"`
	AltHosts        []string `json:"alt_hosts"`
	Compression     bool     `json:"compression"`
	TickerCommitBuf int      `json:"ticker_commit_buffer"`
	TradeCommitBuf  int      `json:"trade_commit_buffer"`
	Level2CommitBuf  int      `json:"level2_commit_buffer"`
	OrdersBookCommitBuf  int  `json:"orders_book_commit_buffer"`
}

// S3 contains config values for s3.
type S3 struct {
	AWSRegion           string `json:"aws_region"`
	AccessKeyID         string `json:"access_key_id"`
	SecretAccessKey     string `json:"secret_access_key"`
	Bucket              string `json:"bucket"`
	UsePrefixForObjName bool   `json:"use_prefix_for_object_name"`
	ReqTimeoutSec       int    `json:"request_timeout_sec"`
	MaxIdleConns        int    `json:"max_idle_conns"`
	MaxIdleConnsPerHost int    `json:"max_idle_conns_per_host"`
	TickerCommitBuf     int    `json:"ticker_commit_buffer"`
	TradeCommitBuf      int    `json:"trade_commit_buffer"`
}

// Log contains config values for logging.
type Log struct {
	Level    string `json:"level"`
	FilePath string `json:"file_path"`
}
