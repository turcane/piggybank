package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
)

var config configuration

type balance struct {
	eur float64
	btc float64
}

type configuration struct {
	APIKey              string
	PrivateKey          string
	WithdrawAddressDesc string
}

func main() {
	setupConfig()
	api := krakenapi.New(config.APIKey, config.PrivateKey)
	for {
		print("Checking Balance ...")
		balance, err := getBalance(api)
		if err != nil {
			print("Could not check Balance. Error: " + err.Error())
		} else {
			print(fmt.Sprintf("EUR Balance: %.2f", balance.eur))
			print(fmt.Sprintf("BTC Balance: %.8f", balance.btc))

			if balance.eur >= 50 || balance.btc >= 0.0055 {
				if balance.eur >= 50 {
					err = buyBitcoin(api, balance.eur)
					if err != nil {
						print("Could not buy Bitcoin. Error: " + err.Error())
					}
				} else if balance.btc >= 0.0055 {
					err = withdrawBitcoin(api, balance.btc)
					if err != nil {
						print("Could not withdraw Bitcoin. Error: " + err.Error())
					}
				}
			} else {
				print("Not enough balance found.")
			}
		}
		print("Going to sleept for 1 Hour.")
		time.Sleep(1 * time.Hour)
	}
}

func setupConfig() {
	print("Welcome to Kraken PiggyBank v1.0")

	configFile, err := os.Open("./config.json")
	if err != nil {
		print("Could not open config.json. Exiting...")
		os.Exit(1)
	}

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&config)

	if err != nil {
		print("Could not parse config.json. Error: " + err.Error())
		os.Exit(1)
	}
}

func print(message string) {
	layout := "Jan 06 15:04:05"
	fmt.Println("[", time.Now().Format(layout), "] ", message)
}

func buyBitcoin(api *krakenapi.KrakenApi, balance float64) error {
	print("Found some EUR Balance.")
	print("Checking Bitcoin Price.")
	price, err := getBitcoinPrice(api)
	if err != nil {
		return err
	}
	print(fmt.Sprint("Price is at ", price, " € per Bitcoin."))
	buyValue := balance / price
	print(fmt.Sprintf("Going to buy %.5f Bitcoin", buyValue))
	args := make(map[string]string)
	args["expiretm"] = "+3300"          // 55 Minutes
	args["trading_agreement"] = "agree" // Needed for german accounts

	print(fmt.Sprintf("Creating an Order with %.2f € to buy aprox. %.5f Bitcoin.", balance, buyValue))
	print(fmt.Sprintf("Order Expiry: 55 Minutes."))

	r, err := api.AddOrder("XXBTZEUR", "buy", "market", fmt.Sprintf("%.5f", buyValue), args)
	if err != nil {
		return err
	}

	print(fmt.Sprintf("Order was created successfully. TX IDs: %v", r.TransactionIds))

	return err
}

func withdrawBitcoin(api *krakenapi.KrakenApi, balance float64) error {
	print(fmt.Sprintf("Withdrawing %.5f Bitcoin to %s", balance, config.WithdrawAddressDesc))
	api.Withdraw("XBT", config.WithdrawAddressDesc, new(big.Float).SetFloat64(balance))
	return nil
}

func getBalance(api *krakenapi.KrakenApi) (balance, error) {
	b, err := api.Balance()
	if err != nil {
		return balance{eur: 0, btc: 0}, err
	}
	return balance{eur: b.ZEUR, btc: b.XXBT}, err
}

func getBitcoinPrice(api *krakenapi.KrakenApi) (float64, error) {
	t, err := api.Ticker("XBTEUR")
	if err != nil {
		return 0, err
	}

	price, err := strconv.ParseFloat(t.XXBTZEUR.Ask[0], 64)
	if err != nil {
		return 0, err
	}
	return price, err
}
