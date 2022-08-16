package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	_ "github.com/ClickHouse/clickhouse-go"
	_ "github.com/go-sql-driver/mysql"
	jsoniter "github.com/json-iterator/go"
	"github.com/milkywaybrain/cryptogalaxy/internal/config"
	"github.com/milkywaybrain/cryptogalaxy/internal/initializer"
)

func main() {

	// Load config file values.
	// Default path for file is ./config.json.
	cfgPath := flag.String("config", "./config.json", "configuration JSON file path")
	flag.Parse()
	cfgFile, err := os.Open(*cfgPath)
	if err != nil {
		fmt.Println("Not able to find config file :", *cfgPath)
		fmt.Println("exiting the app")
		return
	}
	var cfg config.Config
	if err = jsoniter.NewDecoder(cfgFile).Decode(&cfg); err != nil {
		fmt.Println("Not able to parse JSON from config file :", *cfgPath)
		fmt.Println("exiting the app")
		return
	}
	cfgFile.Close()

	// Start the app.
	err = initializer.Start(context.Background(), &cfg)
	if err != nil {
		fmt.Println(err)
		fmt.Println("exiting the app")
	}
}
