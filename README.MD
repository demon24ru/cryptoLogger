## Cryptogalaxy
 
Cryptogalaxy is an app which will get any cryptocurrencies ticker and trade data in real time from multiple exchanges and then saves it in multiple storage systems.

<p>Currently supported exchanges :<br></p>

<div>1. <img src="./docs/exchange/ftx.png" title="FTX" height="24" width="24"/> FTX</div>
<div>2. <img src="./docs/exchange/coinbase-pro.png" title="Coinbase Pro" height="24" width="24"/> Coinbase Pro</div>
<div>3. <img src="./docs/exchange/binance.png" title="Binance" height="24" width="24"/> Binance</div>
<div>4. <img src="./docs/exchange/bitfinex.png" title="Bitfinex" height="24" width="24"/> Bitfinex</div>
<div>5. <img src="./docs/exchange/huobi.png" title="Huobi" height="24" width="24"/> Huobi</div>
<div>6. <img src="./docs/exchange/gateio.png" title="Gateio" height="24" width="24"/> Gateio</div>
<div>7. <img src="./docs/exchange/kucoin.png" title="Kucoin" height="24" width="24"/> Kucoin</div>
<div>8. <img src="./docs/exchange/bitstamp.png" title="Bitstamp" height="24" width="24"/> Bitstamp</div>
<div>9. <img src="./docs/exchange/bybit.png" title="Bybit" height="24" width="24"/> Bybit</div>
<div>10. <img src="./docs/exchange/probit.png" title="Probit" height="24" width="24"/> Probit</div>
<div>11. <img src="./docs/exchange/gemini.png" title="Gemini" height="24" width="24"/> Gemini</div>
<div>12. <img src="./docs/exchange/bitmart.png" title="Bitmart" height="24" width="24"/> Bitmart</div>
<div>13. <img src="./docs/exchange/digifinex.png" title="Digifinex" height="24" width="24"/> Digifinex</div>
<div>14. <img src="./docs/exchange/ascendex.png" title="AscendEX" height="24" width="24"/> AscendEX</div>
<div>15. <img src="./docs/exchange/kraken.png" title="Kraken" height="24" width="24"/> Kraken</div>
<div>16. <img src="./docs/exchange/binance-us.png" title="Binance US" height="24" width="24"/> Binance US</div>
<div>17. <img src="./docs/exchange/okex.png" title="OKEx" height="24" width="24"/> OKEx</div>
<div>18. <img src="./docs/exchange/ftx-us.png" title="FTX US" height="24" width="24"/> FTX US</div>
<div>19. <img src="./docs/exchange/hitbtc.png" title="HitBTC" height="24" width="24"/> HitBTC</div>
<div>20. <img src="./docs/exchange/aax.png" title="AAX" height="24" width="24"/> AAX</div>
<div>21. <img src="./docs/exchange/bitrue.png" title="Bitrue" height="24" width="24"/> Bitrue</div>
<div>22. <img src="./docs/exchange/btse.png" title="BTSE" height="24" width="24"/> BTSE</div>
<div>23. <img src="./docs/exchange/mexo.png" title="Mexo" height="24" width="24"/> Mexo</div>
<div>24. <img src="./docs/exchange/bequant.png" title="BEQUANT" height="24" width="24"/> BEQUANT</div>
<div>25. <img src="./docs/exchange/lbank.png" title="LBank" height="24" width="24"/> LBank</div>
<div>26. <img src="./docs/exchange/coinflex.png" title="CoinFLEX" height="24" width="24"/> CoinFLEX</div>
<div>27. <img src="./docs/exchange/binance.png" title="Binance TR" height="24" width="24"/> Binance TR</div>
<div>28. <img src="./docs/exchange/cryptodot-com.png" title="Crypto.com Exchange" height="24" width="24"/> Crypto.com Exchange</div>
<div>29. <img src="./docs/exchange/fmfwio.png" title="FMFW.io" height="24" width="24"/> FMFW.io</div>
<div>30. <img src="./docs/exchange/changelly-pro.png" title="Changelly PRO" height="24" width="24"/> Changelly PRO</div>
 
<p><br>Currently supported storages :<br></p>

<div>1. <img src="./docs/storage/terminal.png" title="Terminal Output" height="24" width="24"/> Terminal Output</div>
<div>2. <img src="./docs/storage/mysql.png" title="MySQL" height="24" width="24"/> MySQL</div>
<div>3. <img src="./docs/storage/elasticsearch.png" title="Elasticsearch" height="24" width="24"/> Elasticsearch</div>
<div>4. <img src="./docs/storage/influxdb.svg" title="InfluxDB" height="24" width="24"/> InfluxDB</div>
<div>5. <img src="./docs/storage/nats.png" title="NATS" height="24" width="24"/> NATS</div>
<div>6. <img src="./docs/storage/clickhouse.jpeg" title="ClickHouse" height="24" width="24"/> ClickHouse</div>
<div>7. <img src="./docs/storage/s3.png" title="S3" height="24" width="24"/> S3</div>
 
---------------------------------------  
 * [Features](#features)
 * [Limitations](#limitations)
 * [Why the app](#why-the-app)
 * [Should you use it](#should-you-use-it)
 * [Who uses Cryptogalaxy](#who-uses-cryptogalaxy)
 * [Install and run](#install-and-run)
 * [Architecture](#architecture)
 * [Configuration options](#configuration-options)
 * [Storage schema](#storage-schema)
 * [Output screenshots](#output-screenshots)
 * [Testing](#testing)
 * [Module dependency](#module-dependency)
 * [Contributing](#contributing)
 * [License](#license)
 * [Donate](#donate)
---------------------------------------
 
## Features
 
* Easy configuration of the app via JSON file.
 
* Supports both REST and websocket as a way to fetch data.
 
* All the data received from multiple exchanges are converted into common format and then displayed in terminal or stored to MySQL, Elasticsearch, InfluxDB, NATS, ClickHouse and S3.
 
* Some exchanges impose a rate limit on the number of channel subscriptions sent to websocket connections per second and if it crosses then disconnects the connection. To avoid this, the app makes staggered requests based on the rate limit of specific exchange.
 
* Users can define intervals for each REST API request. 
 
* Users can also define data consideration intervals for websocket, so that heavy influx of stream data can be filtered on a time basis.
 
* If there is any problem with the connection, the app retries with a 1 minute time gap upto 10 times before failing. Retry counter will be reset back to 0 if the last retry happened 10 minutes ago. And also, all these retry parameter values can be changed through the config per exchange wise. 
 
* Users can define buffer size for data in memory before committing to different storage systems.
 
* Use of concurrency primitives for faster and efficient execution.
 
* Request timeout can be set to every websocket read, write connections and REST API.
 
* Connection pooling for MySQL, Elasticsearch, InfluxDB, NATS, ClickHouse and S3 commits.
 
* Detailed logging of app execution steps, errors to a file for easy debugging.
 
## Limitations
 
* Supports only spot markets.
 
*Note :* Although some exchanges like FTX, do not care for spot or future markets in their API, the app was not tested for other than spot markets.
 
* App creates a single websocket connection per exchange. So, you may hit the threshold of no. of markets you want to get the data from it. In that case you need to run a new instance of the app for more markets.
 
*Note :* Out of 30 currently supported exchanges, this is a real problem only in Bitfinex which is restricting 25 public channel subscriptions per websocket connection. So if you want to subscribe to more than 25 markets data from Bitfinex through websocket, then you need to have one more instance of the app running.
 
## Why the app
 
For some personal projects I wanted to have cryptocurrency data from multiple exchanges. I researched and did not get any suitable library or service which matched my exact requirements and was also cost effective. So I created this app. 
 
Once the app was in its final stage, I thought it may help other developers who want the same set of data. So open sourcing it. 
 
## Should you use it
 
If you already have a script / library / service to get cryptocurrencies data and it is working fine then probably you should continue with that. Else if you have an idea for displaying / utilizing cryptocurrencies data in a beautiful way and have not yet decided on the way of getting it then you should try it.

## Who uses Cryptogalaxy

* Moon or Earth
 
Twitter bot for getting various metrics of cryptocurrencies.

[https://twitter.com/moon_or_earth](https://twitter.com/moon_or_earth)

*Note :* If you have done something wonderful using the app and you are OK with sharing the details with the public, please give a pull request or send me an email. I will include the link for the same over here.
 
## Install and run 
 
You can install and run the app in any one of the following way :
 
**1.** Download the binaries from the [releases page](https://github.com/milkywaybrain/cryptogalaxy/releases). Then you can run it like,
 
```
cryptogalaxy
```
if you copied the release binary to a location in `PATH` and the config file is located in the same directory.
 
OR
 
```
./cryptogalaxy -config=${CONFIGURATION_FILE_PATH}
```
from any other location of the binary with the defined path of config file.
 
***Example quick install for Linux :***
 
```
curl -Ls https://api.github.com/repos/milkywaybrain/cryptogalaxy/releases/latest \
| grep -wo "https.*linux_amd64*.tar.gz" \
| wget -qi - \
&& tar -xf cryptogalaxy*.tar.gz \
&& chmod +x ./cryptogalaxy \
&& sudo mv cryptogalaxy /usr/local/bin/
```
and then,
```
cryptogalaxy
```
 
*Note :* Same can be done for mac and windows systems. For mac, please download the archive with the name containing `darwin` in it.
 
**2.** Download the source, run it using [Go tools](https://golang.org/).
 
```
go run ${APP_PATH}/cmd/cryptogalaxy/ -config=${CONFIGURATION_FILE_PATH}
```
 
**3.** If you have an [Go app](https://golang.org/), include Cryptogalaxy using command :
 
```
go get -u github.com/milkywaybrain/cryptogalaxy
```
 
*Note :*  
 
* Default path for the configuration file is ./config.json. 
 
* Example configuration file can be found at [./examples/config.json](./examples/config.json) and it's details at [Configuration options](#configuration-options). 
 
* If you want to store data in MySQL or Elasticsearch or InfluxDB or NATS or ClickHouse or in S3 along with a terminal display, then please install those systems separately.
 
## Architecture
 
Following diagram summarizes the architecture of the app which is written in Go programming language. Diagrams raw `.drawio` file can be found at [./docs/architecture.drawio](./docs/architecture.drawio)
 
<p align="center">
<img align="center" src="./docs/architecture.png" />
</p>
 
## Configuration options
 
Configuration format for the app is JSON.
 
Example configuration file can be found at [./examples/config.json](./examples/config.json)
 
```json
{
    "exchanges": [
        {
            "name": "ftx",
            "markets": [
                {
                    "id": "BTC/USD",
                    "info": [
                        {
                            "channel": "ticker",
                            "connector": "rest",
                            "rest_ping_interval_sec": 10,
                            "storages": [
                                "terminal",
                                "mysql",
                                "elastic_search",
                                "influxdb",
                                "nats",
                                "clickhouse",
                                "s3"
                            ]
                        },
                        {
                            "channel": "trade",
                            "connector": "websocket",
                            "websocket_consider_interval_sec": 0,
                            "storages": [
                                "mysql",
                                "elastic_search",
                                "influxdb",
                                "nats",
                                "clickhouse",
                                "s3"
                            ]
                        }
                    ],
                    "commit_name": ""
                }
            ],
            "retry": {
                "number": 10,
                "gap_sec": 60,
                "reset_sec": 600
            }
        }
    ],
    "connection": {
        "websocket": {
            "conn_timeout_sec": 10,
            "read_timeout_sec": 0
        },
        "rest": {
            "request_timeout_sec": 10,
            "max_idle_conns": 100,
            "max_idle_conns_per_host": 10
        },
        "terminal": {
            "ticker_commit_buffer": 1,
            "trade_commit_buffer": 1
        },
        "mysql": {
            "user": "root",
            "password": "admin",
            "URL": "@tcp(127.0.0.1:3306)",
            "schema": "cryptogalaxy",
            "request_timeout_sec": 10,
            "conn_max_lifetime_sec": 180,
            "max_open_conns": 10,
            "max_idle_conns": 10,
            "ticker_commit_buffer": 100,
            "trade_commit_buffer": 100
        },
        "elastic_search": {
            "addresses": [
                "http://localhost:9200/"
            ],
            "username": "",
            "password": "",
            "index_name": "cryptogalaxy",
            "request_timeout_sec": 10,
            "max_idle_conns": 10,
            "max_idle_conns_per_host": 10,
            "ticker_commit_buffer": 100,
            "trade_commit_buffer": 100
        },
        "influxdb": {
            "organization": "crypto",
            "bucket": "cryptogalaxy",
            "token": "t3Z1bFhjcUo_p9j_6_vuqga7_NQp9UZPdA6YZNNpzY3NUZM4GmRkN80fjHJDMIYA9kfA4GRUoOtty7bpdFoXfQ==",
            "URL": "http://localhost:8086/",
            "request_timeout_sec": 10,
            "max_idle_conns": 10,
            "ticker_commit_buffer": 100,
            "trade_commit_buffer": 100
        },
        "nats": {
            "addresses": [
                "http://localhost:4222/"
            ],
            "username": "",
            "password": "",
            "subject_base_name": "cryptogalaxy",
            "request_timeout_sec": 10,
            "ticker_commit_buffer": 100,
            "trade_commit_buffer": 100
        },
        "clickhouse": {
            "user": "",
            "password": "",
            "URL": "tcp://127.0.0.1:9000",
            "schema": "cryptogalaxy",
            "request_timeout_sec": 10,
            "alt_hosts": [
                "127.0.0.2:9000",
                "127.0.0.3:9000"
            ],
            "compression": false,
            "ticker_commit_buffer": 100,
            "trade_commit_buffer": 100
        },
        "s3": {
            "aws_region": "us-west-2",
            "access_key_id": "AKIAIOSFODNN7EXAMPLE",
            "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
            "bucket": "cryptogalaxy",
            "use_prefix_for_object_name": false,
            "request_timeout_sec": 10,
            "max_idle_conns": 10,
            "max_idle_conns_per_host": 10,
            "ticker_commit_buffer": 100,
            "trade_commit_buffer": 100
        }
    },
    "log": {
        "level": "error",
        "file_path": "/var/log/cryptogalaxy/cryptogalaxy"
    }
}
```
 
**Details of the configuration options**
 
*Note :* All values in the configuration are case sensitive.
 
***Exchange channel settings*** :
 
* **exchanges : name** : Name of the exchange from which you need data.
 
Possible values : ftx, coinbase-pro, binance, bitfinex, huobi, gateio, kucoin, bitstamp, bybit, probit, gemini, bitmart, digifinex, ascendex, kraken, binance-us, okex, ftx-us, hitbtc, aax, bitrue, btse, mexo, bequant, lbank, coinflex, binance-tr, cryptodot-com, fmfwio, changelly-pro.
 
* **exchanges : markets : id** : Market symbol for which data is needed. This has to be the exact one defined by exchange.
 
Possible values : You can directly consult the exchange doc page for the exact symbol or else you can look up to [./examples/markets.csv](./examples/markets.csv) file for all the exchange symbols.
 
*Note :* If you have done app setup by source code, then you can regenerate [./examples/markets.csv](./examples/markets.csv) file by running following script, which queries all the exchange for all supported symbols :
 
```
go run ${APP_PATH}/scripts/markets.go
```
 
* **exchanges : markets : info : channel** : Market channel for which data is needed.
 
Possible values : ticker, trade.
 
*Note :* Some exchanges do not give trade id in the trade data response, in that case trade id will be zero.
 
* **exchanges : markets : info : connector** : How you want to get the data from exchange.
 
Possible values : websocket, rest
 
*Note :* As the app fetches only the last 100 trades (or default one incase limit is not supported) from REST API due to the max limit set by most exchanges, data may contain duplicate entries if the configured exchanges : markets : info : rest_ping_interval_sec is too small or misses data if the configured value is too high. So, for trade data it is always better to use websocket as a connector.
 
* **exchanges : markets : info : websocket_consider_interval_sec** : If you connect to a websocket channel, then you will get a stream of data continuously. Sometimes it makes sense to ignore some of this to have a good set of data. So this flag enables that. It considers stream data only on specified intervals, and ignores all the other time.
 
Possible values : 0, if you want all data all time, greater than 0 sec for any other time. 
 
*Note :* Some exchanges send ticker data with a time gap even in websocket connection, so please experiment with this configuration value. And also if you need ticker data only on a specified interval, like say every minute, then you can also consider using the REST API.
 
* **exchanges : markets : info : rest_ping_interval_sec** : It tells which interval app should make REST API calls to get the data.
 
Possible values : greater than 0 sec.
 
*Note :* Every exchange has a rate limit for REST API calls. So please configure this considering the limit, otherwise your connection may be refused. Better to use websocket if you need real time data.
 
* **exchanges : markets : info : storages** : Storage systems on which data need to be stored.
 
Possible values : terminal, mysql, elastic_search, influxdb, nats, clickhouse, s3.
 
*Note :* Terminal option is only used to check the price on terminal display, it is not persistent.
 
* **exchanges : markets : commit_name** : Every exchange has different symbols for the same market, so if you want to generalize that and save only common names in storage systems you can use this. For example, you can give the "BTC/USDT" name for the BTC USDT pair of all exchanges so that the storage system stores the market symbol as "BTC/USDT" for all the exchange.
 
Possible values : generalized name or empty string if you don't need it.
 
* **exchanges : retry : number** : Number of times exchange functions should be retried on any error, before failing.
 
Possible values : 0 for no retry, greater than 0 for any other number.
 
*Note :* Once any exchange fails after a configured number of retry, then all the other exchanges will be shut down and app is made to exit.
 
* **exchanges : retry : gap_sec** : Time gap for each retry.
 
Possible values : 0 for no gap, greater than 0 sec for any other time.
 
* **exchanges : retry : reset_sec** : Time elapsed since the last retry after which retry number should be reset back to 0.
 
Possible values : 0 for no reset, greater than 0 sec for any other time. 
 
***Websocket connection settings*** : 
 
These options are needed only if you want to connect to the exchange through websocket.
 
* **connection : websocket : conn_timeout_sec** : Websocket connection timeout.
 
Possible values : 0 for no timeout, greater than 0 sec for any other time.
 
* **connection : websocket : read_timeout_sec** : Websocket read timeout (data read).
 
Possible values : 0 for no timeout, greater than 0 sec for any other time. 
 
***REST connection settings*** : 
 
These options are needed only if you want to connect exchanges through REST API.
 
* **connection : rest : request_timeout_sec** : REST API request timeout.
 
Possible values : 0 for no timeout, greater than 0 sec for any other time.
 
* **connection : rest : max_idle_conns** : Maximum number of idle (keep-alive) connections across all hosts for REST API.
 
Possible values : 0 for no limit, greater than 0 for any other number.
 
* **connection : rest : max_idle_conns_per_host** : Maximum idle (keep-alive) connections to keep per-host for REST API.
 
Possible values : 0 for 2 (this may change in future), greater than 0 for any other number. 
 
***Terminal display settings*** : 
 
These options are needed only if you want to display data in the terminal.
 
* **connection : terminal : ticker_commit_buffer** : Size of market tickers to be buffered in memory before displaying data in the terminal.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10.
 
* **connection : terminal : trade_commit_buffer** : Size of market trades to be buffered in memory before displaying data in terminal.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 
 
***MySQL settings*** : 
 
These options are needed only if you want to store data in mysql.
 
* **connection : mysql : user** : Username for database.
 
* **connection : mysql : password** : Password for database.
 
* **connection : mysql : URL** : URL of the database.
 
* **connection : mysql : schema** : Schema name of the database.
 
* **connection : mysql : request_timeout_sec** : Timeout for MySQL connection and insert data.
 
Possible values : 0 for no timeout, greater than 0 for any other time.
 
* **connection : mysql : conn_max_lifetime_sec** : It is required to ensure connections are closed by the MySQL driver safely before connection is closed by MySQL server, OS, or other middlewares. Since some middlewares close idle connections by 5 minutes, MySQL driver for Go, which is used by the app, recommends a timeout shorter than 5 minutes. This setting helps load balancing and changing system variables too.
 
Possible values : 0, connections are not closed due to a connection's age. Greater than 0 sec for any other time.
 
* **connection : mysql : max_open_conns** : It is highly recommended to limit the number of connections used by the application. This depends on the application and MySQL server.
 
Possible values : 0, no limit on the number of open connections. Greater than 0 for any other number.
 
* **connection : mysql : max_idle_conns** : It is recommended to be set the same to (or greater than) mysql : max_open_conns. When it is smaller than mysql : max_open_conns, connections can be opened and closed very frequently than you expect. Idle connections can be closed by the mysql : conn_max_lifetime_sec.
 
Possible values : 0 for 2 (this may change in future), greater than 0 for any other number.
 
* **connection : mysql : ticker_commit_buffer** : Size of market tickers to be buffered in memory before inserting data to MySQL.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 
 
* **connection : mysql : trade_commit_buffer** : Size of market trades to be buffered in memory before inserting data to MySQL.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 
 
***Elasticsearch settings*** : 
 
These options are needed only if you want to store data in Elasticsearch.
 
* **connection : elastic_search : addresses** : A list of Elasticsearch nodes to use.
 
* **connection : elastic_search : username** : Username for Elasticsearch HTTP basic authentication.
 
Possible values : value or empty string if there is no authentication.
 
* **connection : elastic_search : password** : Password for Elasticsearch HTTP basic authentication.
 
Possible values : value or empty string if there is no authentication.
 
* **connection : elastic_search : index_name** : Index name of the Elasticsearch.
 
* **connection : elastic_search : request_timeout_sec** : Timeout for Elasticsearch connection and index data.
 
Possible values : 0 for no timeout, greater than 0 sec for any other time.
 
* **connection : elastic_search : max_idle_conns** : Maximum number of idle (keep-alive) connections across all hosts for Elasticsearch.
 
Possible values : 0 for no limit, greater than 0 for any other number.
 
* **connection : elastic_search : max_idle_conns_per_host** : Maximum idle (keep-alive) connections to keep per-host for Elasticsearch.
 
Possible values : 0 for 2 (this may change in future), greater than 0 for any other number.
 
* **connection : elastic_search : ticker_commit_buffer** : Size of market tickers to be buffered in memory before indexing data to Elasticsearch.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 
 
Also, this has nothing to do with indexing buffer settings available in Elasticsearch, that is different.
 
* **connection : elastic_search : trade_commit_buffer** : Size of market trades to be buffered in memory before indexing data to Elasticsearch.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 
 
Also, this has nothing to do with indexing buffer settings available in Elasticsearch, that is different.

***InfluxDB settings*** : 
 
These options are needed only if you want to store data in influxdb.
 
* **connection : influxdb : organization** : Name for InfluxDB organization.
 
* **connection : influxdb : bucket** : Name for InfluxDB bucket.

* **connection : influxdb : token** : Server authentication token.
 
* **connection : influxdb : URL** : InfluxDB server base URL.

* **connection : influxdb : request_timeout_sec** : InfluxDB API request timeout.
 
Possible values : 0 for no timeout, greater than 0 for any other time.
 
* **connection : influxdb : max_idle_conns** : Maximum number of idle (keep-alive) connections across all hosts for InfluxDB API.
 
Possible values : 0 for no limit, greater than 0 for any other number.
 
* **connection : influxdb : ticker_commit_buffer** : Size of market tickers to be buffered in memory before inserting data to InfluxDB.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 
 
* **connection : influxdb : trade_commit_buffer** : Size of market trades to be buffered in memory before inserting data to InfluxDB.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 

***NATS settings*** : 
 
These options are needed only if you want to store data in NATS.
 
* **connection : nats : addresses** : A list of NATS servers to use.
 
* **connection : nats : username** : Username for NATS basic authentication.
 
Possible values : value or empty string if there is no authentication.
 
* **connection : nats : password** : Password for NATS basic authentication.
 
Possible values : value or empty string if there is no authentication.
 
* **connection : nats : subject_base_name** : Subject base name of the NATS.

*Note :* For ticker the final subject name will be subject_base_name.ticker and for trade it is subject_base_name.trade.
 
* **connection : nats : request_timeout_sec** : Timeout for NATS connection.
 
Possible values : 0 for no timeout, greater than 0 sec for any other time.

* **connection : nats : ticker_commit_buffer** : Size of market tickers to be buffered in memory before publishing data to NATS.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 
 
NATS client library automatically buffers messages before sending them to the server which we cannot configure. So if the ticker_commit_buffer number is very big, then sometimes the data which is first internally buffered in the memory by the Cryptogalaxy app is pushed to the NATS server in chunks by the NATS client library so the last few trades may be still buffered for the next commit. This is not at all the concern, but mentioned here to just as an information to know.
 
* **connection : nats : trade_commit_buffer** : Size of market trades to be buffered in memory before publishing data to NATS.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 
 
NATS client library automatically buffers messages before sending them to the server which we cannot configure. So if the ticker_commit_buffer number is very big, then sometimes the data which is first internally buffered in the memory by the Cryptogalaxy app is pushed to the NATS server in chunks by the NATS client library so the last few trades may be still buffered for the next commit. This is not at all the concern, but mentioned here to just as an information to know.

***ClickHouse settings*** : 
 
These options are needed only if you want to store data in clickhouse.
 
* **connection : clickhouse : user** : Username for database.
 
* **connection : clickhouse : password** : Password for database.
 
* **connection : clickhouse : URL** : URL of the database.
 
* **connection : clickhouse : schema** : Schema name of the database.
 
* **connection : clickhouse : request_timeout_sec** : Timeout for ClickHouse connection and insert data.
 
Possible values : 0 for no timeout, greater than 0 for any other time.

* **connection : clickhouse : alt_hosts** : Comma separated list of single address host for load-balancing.
 
Possible values : list of address or empty array if you don't need it.

* **connection : clickhouse : compression** : Enable lz4 compression.
 
Possible values : boolean true for compression, false for no compression.

* **connection : clickhouse : ticker_commit_buffer** : Size of market tickers to be buffered in memory before inserting data to ClickHouse.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 
 
* **connection : clickhouse : trade_commit_buffer** : Size of market trades to be buffered in memory before inserting data to ClickHouse.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 

***S3 settings*** : 
 
These options are needed only if you want to store data in s3.
 
* **connection : s3 : aws_region** : Where to send requests, such as us-west-2 or us-east-2.
 
* **connection : s3 : access_key_id** : AWS access key ID.
 
* **connection : s3 : secret_access_key** : AWS secret access key.
 
* **connection : s3 : bucket** : S3 bucket name.
 
* **connection : s3 : use_prefix_for_object_name** : Enable 0-9 prefixing for object name in the bucket. For example, if enabled, the object key name would be s3://cryptogalaxy/3/trade/binance1632400443835060152.json or it will be s3://cryptogalaxy/trade/binance1632400443835060152.json.
 
Possible values : boolean true for prefixing, false for no prefix.

* **connection : s3 : request_timeout_sec** : S3 API request timeout.
 
Possible values : 0 for no timeout, greater than 0 sec for any other time.
 
* **connection : s3 : max_idle_conns** : This setting represents the maximum number of idle (keep-alive) connections across all hosts. One use case for increasing this value is when you are seeing many connections in a short period from the same clients.
 
Possible values : 0 for no limit, greater than 0 for any other number.
 
* **connection : s3 : max_idle_conns_per_host** : This setting represents the maximum number of idle (keep-alive) connections to keep per-host. One use case for increasing this value is when you are seeing many connections in a short period from the same clients.
 
Possible values : 0 for 2 (this may change in future), greater than 0 for any other number. 

* **connection : s3 : ticker_commit_buffer** : Size of market tickers to be buffered in memory before inserting data to S3. Each commit to S3 will create a new object in the specified bucket.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 
 
* **connection : s3 : trade_commit_buffer** : Size of market trades to be buffered in memory before inserting data to S3. Each commit to S3 will create a new object in the specified bucket.
 
Possible values : > 0
 
*Note :* All REST API and websocket methods buffers data locally and then commits.
So for example, if the specified number is 10 and has multiple market data points then each corresponding method commits data once it reaches the buffer size limit 10 and not the time all methods combined buffer reaches cumulative size of 10. 

***Log settings*** :
 
* **log : level** : App logging level.
 
Possible values : error, info, debug.
 
* **log : file_path** : Logger file path.
 
*Note :* If the path ends with ".log", then the app just creates / opens that file for log writing. Otherwise, each time the app starts, it creates a new file for logging with a current timestamp attached to its name.
 
## Storage schema
 
**MySQL**
 
Script can be found at [./scripts/mysql_schema.sql](./scripts/mysql_schema.sql).
 
```sql
CREATE TABLE `ticker` (
 `id` bigint unsigned NOT NULL AUTO_INCREMENT,
 `exchange` varchar(32) NOT NULL,
 `market` varchar(32) NOT NULL,
 `price` decimal(64,8) NOT NULL,
 `timestamp` timestamp(3) NOT NULL,
 `created_at` timestamp(3) NOT NULL,
 PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```
 
```sql
CREATE TABLE `trade` (
 `id` bigint unsigned NOT NULL AUTO_INCREMENT,
 `exchange` varchar(32) NOT NULL,
 `market` varchar(32) NOT NULL,
 `trade_id` varchar(64) NULL,
 `side` varchar(8) NOT NULL,
 `size` decimal(64,8) NOT NULL,
 `price` decimal(64,8) NOT NULL,
 `timestamp` timestamp(3) NOT NULL,
 `created_at` timestamp(3) NOT NULL,
 PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```
 
**Elasticsearch** 
 
Script can be found at [./scripts/elastic_search_schema.json](./scripts/elastic_search_schema.json).
 
```json
{
   "mappings": {
       "properties": {
           "channel": {
               "type": "keyword"
           },
           "exchange": {
               "type": "keyword"
           },
           "market": {
               "type": "keyword"
           },
           "trade_id": {
               "type": "keyword"
           },
           "side": {
               "type": "keyword"
           },
           "size": {
               "type": "double"
           },
           "price": {
               "type": "double"
           },
           "timestamp": {
               "type": "date"
           },
           "created_at": {
               "type": "date"
           }
       }
   }
}
```

**InfluxDB**

Line protocol used for writing data to InfluxDB is as follows :

```
ticker,exchange,market price timestamp

trade,exchange,market,side trade_id,size,price timestamp
```
Currently there is no configuration to change the data points. So, if you want to change the tagsets or fieldsets you need to change it in the following file and then build it using [Go tools](https://golang.org/). It is just reordering of names in a single line.

```
${APP_PATH}/internal/storage/influxdb.go
```

*Note :* Sometime, ticker and trade data that we receive from the exchanges will have multiple records for the same timestamp. This data is deleted automatically by the InfluxDB as the system identifies unique data points by their measurement, tag set, and timestamp. Also we cannot add a unique id or timestamp as a new tag to the data set as it may significantly affect the performance of the InfluxDB read / writes. So to solve this problem, here we are adding 1 nanosecond to each timestamp entry of exchange and market combo till it reaches 1 millisecond to have a unique timestamp entry for each data point. This will not change anything as we are maintaining only millisecond precision ticker and trade records. Of course this will break if we have more than a million trades per millisecond per market in an exchange. But we are excluding that scenario.

**NATS**

Two subjects to be published in the NATS server in JSON format. One is for ticker data and another one is for trade data. Structures for the same are as follows :

```
configured_subject_base_name.ticker

Exchange
MktID
MktCommitName
Price
Timestamp

configured_subject_base_name.trade

Exchange
MktID
MktCommitName
TradeID
Side
Size
Price
Timestamp
```

**ClickHouse**
 
Script can be found at [./scripts/clickhouse_schema.sql](./scripts/clickhouse_schema.sql).
 
```sql
CREATE TABLE ticker
(
    `exchange` String,
    `market` String,
    `price` Float64,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
ORDER BY (exchange, market, timestamp);
```
 
```sql
CREATE TABLE trade
(
    `exchange` String,
    `market` String,
    `trade_id` String,
    `side` String,
    `size` Float64,
    `price` Float64,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
ORDER BY (exchange, market, side, timestamp);
```
*Note :* Above table creation schema is a basic simple one. ClickHouse provides a lot of important configurations via create statement. You can choose a different table and database engine, can do data partition having ‘hot’ and ‘cold’ split, schedule old data deletion, configure sparse indexing structure with data replication etc and all through the simple create schema statements. So please go through the ClickHouse official document and customize the above script specific to your needs. If you want you can also change Float64 to Decimal column type to avoid floating arithmetic error in some use cases (of course overflow checks lead to operations slowdown, but then there is an option to turn it off also!). Only thing to remember is, you should not change the ticker and trade table column names. 

**S3**

Two objects to be published in the S3 bucket in JSON array format. One is for ticker data and another one is for trade data. Structures for the same are as follows :

```
s3_bucket_name/ticker/exchange_name_UNIX_NANOSECOND_TIMESTAMP
ex : s3://cryptogalaxy/ticker/binance1632400443835060152.json

OR if "connection:s3:use_prefix_for_object_name" set to 'true',

s3_bucket_name/random [0-9]/ticker/exchange_name_UNIX_NANOSECOND_TIMESTAMP
ex : s3://cryptogalaxy/3/ticker/binance1632400443835060152.json

[{
Exchange
MktID
MktCommitName
Price
Timestamp
}]

s3_bucket_name/trade/exchange_name_UNIX_NANOSECOND_TIMESTAMP
ex : s3://cryptogalaxy/trade/binance1632400443835060152.json

OR if "connection:s3:use_prefix_for_object_name" set to 'true',

s3_bucket_name/random [0-9]/trade/exchange_name_UNIX_NANOSECOND_TIMESTAMP
ex : s3://cryptogalaxy/3/trade/binance1632400443835060152.json

[{
Exchange
MktID
MktCommitName
TradeID
Side
Size
Price
Timestamp
}]
```

## Output screenshots
 
**Terminal :** 
 
For enlarged image, please [click here](./docs/terminal-output.png). 
 
<p align="center">
<img align="center" src="./docs/terminal-output.png" />
</p>
 
**MySQL :** 
 
For enlarged image, please [click here](./docs/mysql-output.png). 
 
<p align="center">
<img align="center" src="./docs/mysql-output.png" />
</p>
 
**Elasticsearch :**
 
For enlarged image, please [click here](./docs/elastic_search-output.png).
 
<p align="center">
<img align="center" src="./docs/elastic_search-output.png" />
</p>

**InfluxDB :** 
 
For enlarged image, please [click here](./docs/influxdb-output.png). 
 
<p align="center">
<img align="center" src="./docs/influxdb-output.png" />
</p>

**NATS :** 
 
For enlarged image, please [click here](./docs/nats-output.png). 
 
<p align="center">
<img align="center" src="./docs/nats-output.png" />
</p>

**ClickHouse :** 
 
For enlarged image, please [click here](./docs/clickhouse-output.png). 
 
<p align="center">
<img align="center" src="./docs/clickhouse-output.png" />
</p>

**S3 :** 
 
For enlarged image, please [click here](./docs/s3-output.png). 
 
<p align="center">
<img align="center" src="./docs/s3-output.png" />
</p>
 
## Testing
 
A single test, which is a kind of both unit and integration test is written for the app.
 
If you have done app setup by source code, you can run the test by command :
 
```
cd ${APP_PATH}
go test -v ./test/
```
 
Following steps are executed when you run the test :
 
1. Loads test configuration for the app from ${APP_PATH}/test/config_test.json.
 
2. Truncates data from storage systems. You should have a test database schema, Elasticsearch index, InfluxDB bucket (with delete access), NATS subject, ClickHouse database and S3 bucket configured in config_test.json with the name starting as test.
 
3. Sets file at ${APP_PATH}/test/data_test/ter_storage_test.txt as terminal output inorder to test terminal display functionality.

4. Sets file at ${APP_PATH}/test/data_test/nats_storage_test.txt as nats output inorder to test nats storage functionality.
 
5. Executes the app for 2 minute to get the data from exchanges.
 
6. Cancels app execution and starts testing data.
 
7. For each exchange, it reads data from Terminal file, MySQL, Elasticsearch , InfluxDB, NATS file, ClickHouse and S3 to which app execution step ingested data in the previous step.
 
8. For each exchange, it checks and verifies data stored in different storage is correct against configured one.
 
9. If all the exchange passes the verification then it marks the test as success.
 
*Note :* 

* It only checks if the data is present in the different storage system, not the actual value of it.

* It is always good to test all the exchanges with all the possible combinations of storage systems as specified in the default test configuration file, ${APP_PATH}/test/config_test.json. But having said that, sometimes some exchanges will have zero trades during the app test run so the app does not receive the data and therefore the test fails. Also for all the people it is not possible to get AWS S3 configured. So in all these exceptional cases, modify the test configuration file specific to your code changes and run the test.
 
## Module dependency
 
Shoutout to modules on which this app depends on :

* ClickHouse Driver
 
Golang driver for ClickHouse.

[https://github.com/ClickHouse/clickhouse-go](https://github.com/ClickHouse/clickhouse-go)

* AWS SDK
 
AWS SDK for the Go programming language.

[https://github.com/aws/aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2)
 
* Elasticsearch Client
 
The official Go client for Elasticsearch.

[https://github.com/elastic/go-elasticsearch](https://github.com/elastic/go-elasticsearch)
 
* MySQL Driver
 
Go MySQL Driver is a MySQL driver for Go's (golang) database/sql package.

[https://github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
 
* Gobwas Websocket Library
 
Tiny WebSocket library for Go.

[https://github.com/gobwas/ws](https://github.com/gobwas/ws)
 
* InfluxDB Client
 
InfluxDB 2 Go Client.

[https://github.com/influxdata/influxdb-client-go](https://github.com/influxdata/influxdb-client-go)

* JSON Parser
 
A high-performance 100% compatible drop-in replacement of "encoding/json". 

[https://github.com/json-iterator/go](https://github.com/json-iterator/go)

* NATS Client
 
Golang client for NATS, the cloud native messaging system. 

[https://github.com/nats-io/nats.go](https://github.com/nats-io/nats.go)

* Pkg Errors
 
Simple error handling primitives.

[github.com/pkg/errors](github.com/pkg/errors)
 
* Zerolog Logger
 
Zero Allocation JSON Logger.

[https://github.com/rs/zerolog](https://github.com/rs/zerolog)
 
* Sync Pkg 
 
Go concurrency primitives in addition to the ones provided by the language and "sync" and "sync/atomic" packages.

[golang.org/x/sync](golang.org/x/sync)
 
* GoReleaser Github Action 
 
GitHub Action for GoReleaser. 

[https://github.com/goreleaser/goreleaser-action](https://github.com/goreleaser/goreleaser-action) 
 
* Golangci-lint Github Action 
 
Official GitHub action for golangci-lint from it's authors. 

[https://github.com/golangci/golangci-lint-action](https://github.com/golangci/golangci-lint-action)
 
## Contributing
 
If you find any bug or have code review comments which will increase performance or readability of the app, please raise the issue or send me an email. Happy to correct. Because I believe that in this infinite universe nothing is perfectly structured, even the big galaxies. So I am, literal stardust.
 
Couple of things to note before code contribution :
 
1. Each exchange functionality was deliberately made separate even though this incurred some duplicate code. Reason is easy code readability and maintenance. So, you may need to change code across all exchange functions if you want to alter some core functionality.
 
2. Linter used is Golangci, configuration for the same can be found at [./.golangci.yml](./.golangci.yml). This also has been configured as one of github release workflows. So, it makes sense to run the same linter on local before committing changes to the main branch.
 
3. Test written for the app is some combination of unit and integration test which involves MySQL, Elasticsearch, InfluxDB, NATS, ClickHouse and S3 system, which is difficult to configure as one of github release workflows. So, you need to run the test locally before committing changes to the main branch. 
 
## License 
 
Cryptogalaxy is licensed under the [MIT License](./LICENSE).
 
## Donate
 
This is not a big app. But still, if you find this is useful and want to donate, please do. (Donation is sent directly to me, Pavan Shetty, original author of the app)
 
**Bitcoin** : 1LkR7QwpKqFEd6Gdueeebfun3djLocjtuu  

**Buy Me a Coffee** : [buymeacoffee.com/milkywaybrain](https://www.buymeacoffee.com/milkywaybrain)
  
**Thank you all!** 
