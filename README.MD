## Cryptogalaxy
 
Forked from [Cryptogalaxy](https://github.com/milkywaybrain/cryptogalaxy) is an app which will get any cryptocurrencies ticker and trade data in real time from multiple exchanges and then saves it in multiple storage systems.

<p>Currently supported exchanges :<br></p>

<div><img src="./docs/exchange/kucoin.png" title="Kucoin" height="24" width="24"/> Kucoin</div>
 
<p><br>Currently supported storages :<br></p>

<div>1. <img src="./docs/storage/terminal.png" title="Terminal Output" height="24" width="24"/> Terminal Output</div>
<div>2. <img src="./docs/storage/clickhouse.jpeg" title="ClickHouse" height="24" width="24"/> ClickHouse</div>
 
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
                            "connector": "websocket",
                            "websocket_consider_interval_sec": 0,
                            "storages": [
                                "terminal",
                                "clickhouse"
                            ]
                        },
                        {
                            "channel": "trade",
                            "connector": "websocket",
                            "websocket_consider_interval_sec": 0,
                            "storages": [
                                "terminal",
                                "clickhouse"
                            ]
                        },
                        {
                            "channel": "level2",
                            "connector": "websocket",
                            "websocket_consider_interval_sec": 0,
                            "storages": [
                              "terminal",
                              "clickhouse"
                            ]
                        },
                        {
                            "channel": "ordersbook",
                            "connector": "rest",
                            "rest_ping_interval_sec": 60,
                            "storages": [
                              "terminal",
                              "clickhouse"
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
          "kucoin_key": "",
          "kucoin_secret": "",
          "kucoin_passphrase": ""
        },
        "terminal": {
            "ticker_commit_buffer": 1,
            "trade_commit_buffer": 1,
            "level2_commit_buffer": 1,
            "orders_book_commit_buffer": 1
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
            "trade_commit_buffer": 100,
            "level2_commit_buffer": 100,
            "orders_book_commit_buffer": 100
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


***Log settings*** :
 
* **log : level** : App logging level.
 
Possible values : error, info, debug.
 
* **log : file_path** : Logger file path.
 
*Note :* If the path ends with ".log", then the app just creates / opens that file for log writing. Otherwise, each time the app starts, it creates a new file for logging with a current timestamp attached to its name.
 
## Storage schema
 

**ClickHouse**
 
Script can be found at [./scripts/clickhouse_schema.sql](./scripts/clickhouse_schema.sql).
 
```sql
SET allow_experimental_object_type = 1;
SET output_format_json_named_tuples_as_objects = 1;
```

```sql
CREATE TABLE ticker_ETH-USDT
(
    `data` JSON,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);
```
 
```sql
CREATE TABLE trade_ETH-USDT
(
    `data` JSON,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);
```

```sql
CREATE TABLE level2_ETH-USDT
(
    `data` JSON,
    `timestamp` DateTime64(9, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);
```

```sql
CREATE TABLE ordersbook_ETH-USDT
(
    `sequence` String,
    `bids` JSON,
    `asks` JSON,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);
```
*Note :* Above table creation schema is a basic simple one. ClickHouse provides a lot of important configurations via create statement. You can choose a different table and database engine, can do data partition having ‘hot’ and ‘cold’ split, schedule old data deletion, configure sparse indexing structure with data replication etc and all through the simple create schema statements. So please go through the ClickHouse official document and customize the above script specific to your needs. If you want you can also change Float64 to Decimal column type to avoid floating arithmetic error in some use cases (of course overflow checks lead to operations slowdown, but then there is an option to turn it off also!). Only thing to remember is, you should not change the ticker and trade table column names. 


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
  
**Thank you all!** 


## Docker & Docker-compose

```shell
apt update && apt -y install apt-transport-https ca-certificates curl gnupg2 software-properties-common
curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add -
add-apt-repository "deb https://download.docker.com/linux/debian $(lsb_release -cs) stable"
apt update && apt -y install docker-ce
# Docker-compose
curl -L https://github.com/docker/compose/releases/download/v2.4.1/docker-compose-`uname -s`-`uname -m` -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose
```

## Запуска проекта в Docker контейнере

```shell
# Запуск
docker-compose -f docker-compose.yml up -d --build

# Останов
docker-compose -f docker-compose.yml down

# Удаление остановленных контейнеров
docker system prune -a --volumes

# Запущеные контейнеры их ID
docker ps -a

# Логи контейнера по ID
docker logs <ID>
```

[Docker clickHouse](https://netpoint-dc.com/blog/zapuskaem-server-yandex-clickhouse-v-vide-docker-s-nastroikoi-dlya-obrabotki-bolshogo-obema-dannih/)

```shell
docker run -d --name clickhouse_1 --ulimit nofile=262144:262144 -v /cdnf/db/log:/var/log/clickhouse-server -v /cdnf/db/data:/var/lib/clickhouse yandex/clickhouse-server
docker run -it --rm --link clickhouse_1:clickhouse-server yandex/clickhouse-client --host clickhouse-server
```

Вставить поочередно схемы таблиц из scripts/clickhouse_schema.sql

```shell
docker stop clickhouse_1
docker rm clickhouse_1
docker system prune -a --volumes
```
