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

	// BinanceFuturesWebsocketURL is the binance exchange websocket url.
	BinanceFuturesWebsocketURL = "wss://fstream.binance.com/ws"
	// BinanceFuturesRESTBaseURL is the binance exchange base REST url.
	BinanceFuturesRESTBaseURL = "https://fapi.binance.com/fapi/v1/"

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
	KucoinRESTBaseURL        = "https://api.kucoin.com/api/v1/"
	KucoinRESTV3URL          = "https://api.kucoin.com/api/v3/"
	KucoinFuturesRESTBaseURL = "https://api-futures.kucoin.com/api/v1/"

	// BitstampWebsocketURL is the bitstamp exchange websocket url.
	BitstampWebsocketURL = "wss://ws.bitstamp.net/"
	// BitstampRESTBaseURL is the bitstamp exchange base REST url.
	BitstampRESTBaseURL = "https://www.bitstamp.net/api/v2/"

	// BybitWebsocketURL is the bybit exchange websocket url.
	BybitWebsocketURL = "wss://stream.bybit.com/v5/public"
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

	// PolymarketWebsocketURL is the Polymarket CLOB market (book) websocket url.
	PolymarketWebsocketURL = "wss://ws-subscriptions-clob.polymarket.com/ws/market"
	// PolymarketGammaBaseURL is the Polymarket Gamma REST base url (market metadata / discovery).
	PolymarketGammaBaseURL = "https://gamma-api.polymarket.com/"
	// PolymarketCLOBBaseURL is the Polymarket CLOB REST base url (tick size / resolution).
	PolymarketCLOBBaseURL = "https://clob.polymarket.com/"
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
	// The fields below are Polymarket-only (ignored by other exchanges). They
	// define WHICH events this subject (id) records, by Gamma tags:
	//   GammaTagID    — base Gamma tag queried for discovery (e.g. 1312 Crypto Prices).
	//   RequireTagIDs — event must ALSO carry all of these tags (AND filter).
	//   ExcludeTagIDs — event must carry none of these tags.
	GammaTagID    int   `json:"gamma_tag_id"`
	RequireTagIDs []int `json:"require_tag_ids"`
	ExcludeTagIDs []int `json:"exclude_tag_ids"`
}

// Info contains config values for different market channels.
type Info struct {
	Channel          string   `json:"channel"`
	Connector        string   `json:"connector"`
	WsConsiderIntSec int      `json:"websocket_consider_interval_sec"`
	RESTPingIntSec   int      `json:"rest_ping_interval_sec"`
	Storages         []string `json:"storages"`
	// MarketTypes selects which Polymarket event types to record for this market
	// (e.g. ["ABOVE","RANGE"]). Empty = all supported types. Ignored by non-Polymarket exchanges.
	MarketTypes []string `json:"market_types"`
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
	Polymarket Polymarket `json:"polymarket"`
}

// Polymarket contains config values for the Polymarket connector.
// Zero values fall back to sane defaults inside the connector.
type Polymarket struct {
	// DiscoveryIntSec is how often to poll Gamma for active BTC markets (default 300).
	DiscoveryIntSec int `json:"discovery_interval_sec"`
	// ResolutionIntSec is how often to poll CLOB for resolution of settled markets (default 300).
	ResolutionIntSec int `json:"resolution_interval_sec"`
	// FullBookIntSec is how often to force a REST full-book anchor per token (default 300).
	FullBookIntSec int `json:"full_book_interval_sec"`
	// AutoCreateTables, if true, idempotently creates the polymarket_* tables at startup.
	AutoCreateTables bool `json:"auto_create_tables"`
	// Auto configures the auto-discovery screening mode (pseudo-subject 'AUTO').
	// Strictly additive: leave it out (or set enabled=false) and the connector
	// behaves exactly as before, recording only the configured subjects.
	Auto PolymarketAuto `json:"auto"`
}

// PolymarketAuto contains config values for the Polymarket auto-discovery
// screening mode: the connector scans the WHOLE Polymarket universe via Gamma,
// lightly watches candidates (top-of-book only), runs the screener
// (internal/screener, ported from mm_engine/screener_zero_curvature.py) and
// decides ITSELF which markets to record fully, under the pseudo-subject 'AUTO'.
// Zero values fall back to the defaults noted below.
type PolymarketAuto struct {
	// Enabled turns the whole mode on. Everything else is ignored when false.
	Enabled bool `json:"enabled"`
	// Storages are the storages AUTO rows are written to ("terminal" /
	// "clickhouse"), the same choice the configured subjects make per-market.
	// Default: clickhouse.
	Storages []string `json:"storages"`
	// RecordTrades also records executed trades (polymarket_trade) for promoted
	// markets, not just the CLOB book stream. Default false.
	RecordTrades bool `json:"record_trades"`

	// ScanIntSec is how often to sweep the Gamma universe for new candidates
	// (default 900).
	ScanIntSec int `json:"scan_interval_sec"`
	// PollIntSec is how often to batch-poll candidate top-of-book and run one
	// screener window (default 300 = the canon's 5-minute grid step).
	PollIntSec int `json:"poll_interval_sec"`
	// ObservationWindowSec is the screener observation window and the minimum
	// time a candidate is watched before it can be judged (default 7200 = 2h;
	// the canon validated 2-6h).
	ObservationWindowSec int `json:"observation_window_sec"`

	// MaxRecording is the budget: at most this many auto markets are RECORDING
	// at once (default 50). Overflow blocks NEW promotions only — a market that
	// already started recording is always finished to resolution.
	MaxRecording int `json:"max_recording"`
	// MaxCandidates caps the watch list, bounding the top-of-book poll (default 400).
	MaxCandidates int `json:"max_candidates"`
	// HysteresisK is the number of CONSECUTIVE passing windows required to
	// promote a candidate to RECORDING (default 2).
	HysteresisK int `json:"hysteresis_k"`
	// HysteresisM is the number of consecutive failing windows required to drop
	// out of the pass list (default 1).
	HysteresisM int `json:"hysteresis_m"`

	// ScanMaxPages bounds the universe sweep. Gamma refuses offset pagination
	// past ~2100, so the sweep is ordered by liquidity descending and takes the
	// top ScanMaxPages*100 events (default 20 = top 2000).
	ScanMaxPages int `json:"scan_max_pages"`

	// Coarse filter — applied on Gamma fields alone, no CLOB calls.
	// MinLiquidity is the minimum event liquidity (default 5000).
	MinLiquidity float64 `json:"min_liquidity"`
	// MinVolume24hr is the minimum event 24h volume (default 0 = no floor).
	MinVolume24hr float64 `json:"min_volume_24hr"`
	// MinHoursToExpiry skips markets about to settle (default 6).
	MinHoursToExpiry float64 `json:"min_hours_to_expiry"`
	// MaxHoursToExpiry skips far-dated markets (default 720 = 30 days).
	MaxHoursToExpiry float64 `json:"max_hours_to_expiry"`
	// MaxTickSize skips coarse-grid markets where a "2 tick" spread is huge
	// (default 0.01).
	MaxTickSize float64 `json:"max_tick_size"`

	// FinalPhaseFrac forces in_pass_list=0 over the final fraction of a dated
	// market's lifetime, regardless of metrics (REVIEW.md §118 — the final day
	// of a weekly is where the loss lives). Default 0.10. The state stays
	// RECORDING: the market is still recorded to resolution.
	FinalPhaseFrac float64 `json:"final_phase_frac"`

	// Gates overrides the screener thresholds. Any field left at 0 keeps the
	// canon default (REVIEW.md §122).
	Gates PolymarketGates `json:"gates"`
}

// PolymarketGates overrides the canonical screener gate thresholds. A zero field
// means "use the canon value"; the canon is the default and the only value the
// golden vectors are generated against, so override deliberately.
type PolymarketGates struct {
	MinTwoSided  float64 `json:"min_two_sided"`  // canon 0.9
	MinSpreadMed float64 `json:"min_spread_med"` // canon 2.0 (ticks)
	MinDepthMed  float64 `json:"min_depth_med"`  // canon 50
	MaxJumpRate  float64 `json:"max_jump_rate"`  // canon 0.05
	MaxResStdT   float64 `json:"max_res_std_t"`  // canon 3.0 (ticks)
}

// WS contains config values for websocket connection.
type WS struct {
	ConnTimeoutSec int `json:"conn_timeout_sec"`
	ReadTimeoutSec int `json:"read_timeout_sec"`
}

// REST contains config values for REST API connection.
type REST struct {
	ReqTimeoutSec       int    `json:"request_timeout_sec"`
	MaxIdleConns        int    `json:"max_idle_conns"`
	MaxIdleConnsPerHost int    `json:"max_idle_conns_per_host"`
	KucoinKey           string `json:"kucoin_key"`
	KucoinSecret        string `json:"kucoin_secret"`
	KucoinPassphrase    string `json:"kucoin_passphrase"`
}

// Terminal contains config values for terminal display.
type Terminal struct {
	TickerCommitBuf     int `json:"ticker_commit_buffer"`
	TradeCommitBuf      int `json:"trade_commit_buffer"`
	Level2CommitBuf     int `json:"level2_commit_buffer"`
	OrdersBookCommitBuf int `json:"orders_book_commit_buffer"`
	// MarketCommitBuf is the batch size for Polymarket market-metadata upserts.
	MarketCommitBuf int `json:"market_commit_buffer"`
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
	User                string   `josn:"user"`
	Password            string   `json:"password"`
	URL                 string   `json:"URL"`
	Schema              string   `json:"schema"`
	ReqTimeoutSec       int      `json:"request_timeout_sec"`
	AltHosts            []string `json:"alt_hosts"`
	Compression         bool     `json:"compression"`
	TickerCommitBuf     int      `json:"ticker_commit_buffer"`
	TradeCommitBuf      int      `json:"trade_commit_buffer"`
	Level2CommitBuf     int      `json:"level2_commit_buffer"`
	OrdersBookCommitBuf int      `json:"orders_book_commit_buffer"`
	// MarketCommitBuf is the batch size for Polymarket market-metadata upserts.
	MarketCommitBuf int `json:"market_commit_buffer"`
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
