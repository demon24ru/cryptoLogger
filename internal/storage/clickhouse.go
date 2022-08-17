package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/milkywaybrain/cryptogalaxy/internal/config"
)

// ClickHouse is for connecting and inserting data to ClickHouse.
type ClickHouse struct {
	DB  *sql.DB
	Cfg *config.ClickHouse
}

var clickHouse ClickHouse

// ClickHouse timestamp format.
const clickHouseTimestamp = "2006-01-02 15:04:05.999"
const clickHouseTimestampMicroSec = "2006-01-02 15:04:05.999999999"

// InitClickHouse initializes ClickHouse connection with configured values.
func InitClickHouse(cfg *config.ClickHouse) (*ClickHouse, error) {
	if clickHouse.DB == nil {
		var dataSourceName strings.Builder
		dataSourceName.WriteString(cfg.URL + "?")
		dataSourceName.WriteString("database=" + cfg.Schema)
		dataSourceName.WriteString("&read_timeout=" + fmt.Sprintf("%d", cfg.ReqTimeoutSec) + "&write_timeout=" + fmt.Sprintf("%d", cfg.ReqTimeoutSec))
		if strings.Trim(cfg.User, "") != "" && strings.Trim(cfg.Password, "") != "" {
			dataSourceName.WriteString("&username=" + cfg.User + "&password=" + cfg.Password)
		}
		if cfg.Compression {
			dataSourceName.WriteString("&compress=1")
		}
		prefix := false
		for i, v := range cfg.AltHosts {
			if strings.Trim(v, "") != "" {
				if !prefix {
					dataSourceName.WriteString("&alt_hosts=")
					prefix = true
				}
				if i == len(cfg.AltHosts)-1 {
					dataSourceName.WriteString(v)
				} else {
					dataSourceName.WriteString(v + ",")
				}
			}
		}
		db, err := sql.Open("clickhouse",
			dataSourceName.String())
		if err != nil {
			return nil, err
		}

		err = db.Ping()
		if err != nil {
			return nil, err
		}
		clickHouse = ClickHouse{
			DB:  db,
			Cfg: cfg,
		}
	}
	return &clickHouse, nil
}

// GetClickHouse returns already prepared clickHouse instance.
func GetClickHouse() *ClickHouse {
	return &clickHouse
}

// CommitTickers batch inserts input ticker data to clickHouse.
func (c *ClickHouse) CommitTickers(appCtx context.Context, data []Ticker, marketID string) error {
	tx, err := c.DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO ticker_" + strings.Replace(marketID, "-", "_", 2) + " (data, timestamp) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range data {
		ticker := data[i]
		_, err := stmt.Exec(ticker.Data, ticker.Timestamp.Format(clickHouseTimestamp))
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// CommitTrades batch inserts input trade data to clickHouse.
func (c *ClickHouse) CommitTrades(appCtx context.Context, data []Trade, marketID string) error {
	tx, err := c.DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO trade_" + strings.Replace(marketID, "-", "_", 2) + " (data, timestamp) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range data {
		trade := data[i]
		_, err := stmt.Exec(trade.Data, trade.Timestamp.Format(clickHouseTimestamp))
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// CommitLevel2 batch inserts input level2 data to clickHouse.
func (c *ClickHouse) CommitLevel2(appCtx context.Context, data []Level2, marketID string) error {
	tx, err := c.DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO level2_" + strings.Replace(marketID, "-", "_", 2) + " (data, timestamp) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range data {
		level2 := data[i]
		_, err := stmt.Exec(level2.Data, level2.Timestamp.Format(clickHouseTimestampMicroSec))
		if err != nil {
			return err
		}
		fmt.Printf("Level2 %-15s\n", level2.MktCommitName)
	}
	fmt.Printf("Level2 END %-15s\n\n", marketID)
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// CommitOrdersBook batch inserts input Order Book data to clickHouse.
func (c *ClickHouse) CommitOrdersBook(appCtx context.Context, data []OrdersBook, marketID string) error {
	tx, err := c.DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO ordersbook_" + strings.Replace(marketID, "-", "_", 2) + " (sequence, bids, asks, timestamp) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range data {
		ordersBook := data[i]
		_, err := stmt.Exec(ordersBook.Sequence, ordersBook.Bids, ordersBook.Asks, ordersBook.Timestamp.Format(clickHouseTimestamp))
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
