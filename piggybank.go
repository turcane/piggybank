package main

import (
	"ELRA/tools"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	_ "github.com/mattn/go-sqlite3"
)

var config configuration

type balance struct {
	eur float64
	btc float64
}

type emailTemplate struct {
	SubjectDeposit  string
	MessageDeposit  string
	SubjectWithdraw string
	MessageWithdraw string
}

type emailContent struct {
	subject string
	message string
}

type emailType int

const (
	orderCreated emailType = 1
	withdraw     emailType = 2
)

type configuration struct {
	SleepTimeHours  int
	SMTPServer      string
	SMTPPort        int
	SMTPUserName    string
	SMTPPassword    string
	SMTPSenderName  string
	SMTPSenderEmail string
	Database        string
	UserConfigs     []userconfiguration
}

type userconfiguration struct {
	AccountID                int
	AccountDescription       string
	APIKey                   string
	PrivateKey               string
	WithdrawAddressDesc      string
	MinEURWithdrawBalance    float64
	MinBTCWithdrawBalance    float64
	SendNotificationEmail    bool
	NotificationEmailAddress string
}

type depositInfo struct {
	eurodeposit     float64
	bitcoinprice    float64
	aproxbitcoinrcv float64
	ordertimeout    int64
}

type withdrawInfo struct {
	balance     float64
	fee         float64
	addressdesc string
}

const databaseType = "sqlite3"
const databaseLastBuyTimestamp = "last_buy_timestamp"
const databaseLastBuyInvest = "last_buy_invest"
const databaseLastBuyPrice = "last_buy_price"

func main() {
	setupConfig()
	setupDatabase()
	for {
		for index, userconfig := range config.UserConfigs {
			api := krakenapi.New(userconfig.APIKey, userconfig.PrivateKey)
			print(fmt.Sprintf("Checking Balance of Account \"%s\" [%d/%d]", userconfig.AccountDescription, index+1, len(config.UserConfigs)))
			balance, err := getBalance(api)

			if err != nil {
				print("Could not check Balance. Error: " + err.Error())
			} else {
				print(fmt.Sprintf("EUR Balance: %.2f", balance.eur))
				print(fmt.Sprintf("BTC Balance: %.8f", balance.btc))

				if balance.eur >= userconfig.MinEURWithdrawBalance || balance.btc >= userconfig.MinBTCWithdrawBalance {
					if balance.eur >= userconfig.MinEURWithdrawBalance {
						err = buyBitcoin(api, balance.eur, userconfig)

						if err != nil {
							print("Could not buy Bitcoin. Error: " + err.Error())
						}
					} else if balance.btc >= userconfig.MinBTCWithdrawBalance {
						err = withdrawBitcoin(api, balance.btc, userconfig)
						if err != nil {
							print("Could not withdraw Bitcoin. Error: " + err.Error())
						}
					}
				} else {
					print("Not enough balance found.")
				}
			}
		}
		print(fmt.Sprintf("Going to sleep for %d Hour(s).", config.SleepTimeHours))
		time.Sleep(time.Duration(config.SleepTimeHours) * time.Hour)
	}
}

func setupConfig() {
	print("Welcome to Kraken PiggyBank v1.5")

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

func setupDatabase() {
	if _, err := os.Stat(config.Database); os.IsNotExist(err) {
		print("No database found. Creating...")
		db, err := sql.Open(databaseType, config.Database)
		checkError(err)
		defer db.Close()

		createInvests, err := db.Prepare("CREATE TABLE invest (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, account_id INTEGER NOT NULL, timestamp INTEGER NOT NULL, invest DOUBLE NOT NULL, bitcoin DOUBLE PRECISION NOT NULL, price DOUBLE NOT NULL);")
		tools.CheckError(err)
		createInvests.Exec()

		createTemp, err := db.Prepare("CREATE TABLE temp (type VARCHAR(18) NOT NULL, account_id INTEGER NOT NULL, s_value VARCHAR(255), i_value INTEGER, d_value DOUBLE, PRIMARY KEY (type, account_id));")
		tools.CheckError(err)
		createTemp.Exec()

	} else {
		print("Loading database...")
	}

}

func print(message string) {
	layout := "Jan 06 15:04:05"
	fmt.Println("[", time.Now().Format(layout), "] ", message)

	logfile, err := os.OpenFile("./piggybank.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("[", time.Now().Format(layout), "] ", "Could not open log file.")
	}
	defer logfile.Close()
	_, err = logfile.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format(layout), message))
	if err != nil {
		fmt.Println("[", time.Now().Format(layout), "] ", "Could not open log file.")
	}
}

func sendWithdrawNotificationEmail(userconfig userconfiguration, withdrawinfo withdrawInfo) {
	var depositinfo depositInfo
	depositinfo.aproxbitcoinrcv = 0
	depositinfo.bitcoinprice = 0
	depositinfo.eurodeposit = 0
	depositinfo.ordertimeout = 0
	sendNotificationEmail(userconfig, withdraw, depositinfo, withdrawinfo)
}

func sendDepositNotificationEmail(userconfig userconfiguration, depositinfo depositInfo) {
	var withdrawinfo withdrawInfo
	withdrawinfo.addressdesc = ""
	withdrawinfo.balance = 0
	withdrawinfo.fee = 0
	sendNotificationEmail(userconfig, orderCreated, depositinfo, withdrawinfo)
}

func sendNotificationEmail(userconfig userconfiguration, emailtype emailType, depositinfo depositInfo, withdrawinfo withdrawInfo) {
	if !userconfig.SendNotificationEmail {
		return
	}

	// Email Content
	var emailtemplate emailTemplate
	jsonFile, err := os.Open("./emailtemplate.json")
	if err != nil {
		print("Could not open emailtemplate.json.")
		return
	}
	decoder := json.NewDecoder(jsonFile)
	err = decoder.Decode(&emailtemplate)
	if err != nil {
		print("Could not parse emailtemplate.json. Error: " + err.Error())
		return
	}

	var emailcontent emailContent
	if emailtype == orderCreated {
		emailcontent.subject = emailtemplate.SubjectDeposit
		emailcontent.message = emailtemplate.MessageDeposit
	} else {
		emailcontent.subject = emailtemplate.SubjectWithdraw
		emailcontent.message = emailtemplate.MessageWithdraw
	}

	// Email Headers
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", config.SMTPSenderName, config.SMTPSenderEmail)
	headers["To"] = userconfig.NotificationEmailAddress
	headers["Subject"] = emailcontent.subject

	// Replace placeholders
	emailcontent.message = strings.ReplaceAll(emailcontent.message, "%eurodeposit%", fmt.Sprintf("%.2f", depositinfo.eurodeposit))
	emailcontent.message = strings.ReplaceAll(emailcontent.message, "%account%", userconfig.WithdrawAddressDesc)
	emailcontent.message = strings.ReplaceAll(emailcontent.message, "%bitcoinprice%", fmt.Sprintf("%f", depositinfo.bitcoinprice))
	emailcontent.message = strings.ReplaceAll(emailcontent.message, "%aproxbitcoinrcv%", fmt.Sprintf("%.8f", depositinfo.aproxbitcoinrcv))
	emailcontent.message = strings.ReplaceAll(emailcontent.message, "%ordertimeout%", fmt.Sprintf("%d", depositinfo.ordertimeout))
	emailcontent.message = strings.ReplaceAll(emailcontent.message, "%sats%", fmt.Sprintf("%d", int64((withdrawinfo.balance-withdrawinfo.fee)*100000000)))
	emailcontent.message = strings.ReplaceAll(emailcontent.message, "%bitcoin%", fmt.Sprintf("%.8f", withdrawinfo.balance-withdrawinfo.fee))
	emailcontent.message = strings.ReplaceAll(emailcontent.message, "%addressdesc%", withdrawinfo.addressdesc)
	emailcontent.message = strings.ReplaceAll(emailcontent.message, "%balance%", fmt.Sprintf("%.8f", withdrawinfo.balance))
	emailcontent.message = strings.ReplaceAll(emailcontent.message, "%fee%", fmt.Sprintf("%.8f", withdrawinfo.fee))

	// Craft Email Content
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + emailcontent.message

	// TLS Config
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         config.SMTPServer,
	}

	// SMTP Connection
	smtpConnection, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", config.SMTPServer, config.SMTPPort), tlsconfig)
	if err != nil {
		print(fmt.Sprintf("Could not connect to SMTP Server: %s", err.Error()))
		return
	}
	defer smtpConnection.Close()

	// SMTP Authentication
	smtpAuth := smtp.PlainAuth("", config.SMTPUserName, config.SMTPPassword, config.SMTPServer)
	smtpClient, err := smtp.NewClient(smtpConnection, config.SMTPServer)
	if err != nil {
		print(fmt.Sprintf("Could not connect to SMTP Server: %s", err.Error()))
		return
	}
	defer smtpClient.Close()

	// Setup Email Contents
	if err = smtpClient.Auth(smtpAuth); err != nil {
		print(fmt.Sprintf("Could not auth to SMTP Server: %s", err.Error()))
		return
	}
	if err = smtpClient.Mail(config.SMTPSenderEmail); err != nil {
		print(fmt.Sprintf("Could not set Sender on Notification Email: %s", err.Error()))
		return
	}
	if err = smtpClient.Rcpt(userconfig.NotificationEmailAddress); err != nil {
		print(fmt.Sprintf("Could not set Receiver on Notification Email: %s", err.Error()))
		return
	}
	wc, err := smtpClient.Data()
	if err != nil {
		print(fmt.Sprintf("Could not send Notification Email: %s", err.Error()))
		return
	}
	_, err = wc.Write([]byte(message))
	if err != nil {
		print(fmt.Sprintf("Could not send Notification Email: %s", err.Error()))
		return
	}

	// Send Email
	err = wc.Close()
	if err != nil {
		print(fmt.Sprintf("Could not send Notification Email: %s", err.Error()))
		return
	}
	err = smtpClient.Quit()
	if err != nil {
		print(fmt.Sprintf("Could not send Notification Email: %s", err.Error()))
		return
	}
	print(fmt.Sprintf("Notification Email send to %s for Account \"%s\".", userconfig.NotificationEmailAddress, userconfig.AccountDescription))
}

func buyBitcoin(api *krakenapi.KrakenApi, balance float64, userconfig userconfiguration) error {
	print("Found some EUR Balance.")
	print("Checking Bitcoin Price.")
	price, err := getBitcoinPrice(api)

	if err != nil {
		return err
	}

	print(fmt.Sprint("Price is at ", price, " € per Bitcoin."))
	buyValue := float64(int(balance)) / float64(int(price))
	print(fmt.Sprintf("Going to buy %.5f Bitcoin", buyValue))
	args := make(map[string]string)
	args["expiretm"] = fmt.Sprintf("+%d", (config.SleepTimeHours*60 - 5)) // Sleep Time in Minutes - 5
	args["trading_agreement"] = "agree"                                   // Needed for german accounts

	print(fmt.Sprintf("Creating an Order with %.2f € to buy aprox. %.5f Bitcoin.", balance, buyValue))
	print(fmt.Sprintf(fmt.Sprintf("Order Expiry: %d Minutes.", (config.SleepTimeHours*60 - 5))))

	r, err := api.AddOrder("XXBTZEUR", "buy", "market", fmt.Sprintf("%.5f", buyValue), args)

	if err != nil {
		return err
	}

	print(fmt.Sprintf("Order was created successfully. TX IDs: %v", r.TransactionIds))

	db, err := sql.Open(databaseType, config.Database)
	checkError(err)
	defer db.Close()

	insertTempSettings, err := db.Prepare("INSERT INTO temp (type, account_id, s_value, i_value, d_value) VALUES (?, ?, ?, ?, ?)")
	tools.CheckError(err)
	updateTemp, err := db.Prepare("UPDATE temp SET s_value = ?, i_value = ?, d_value = ? WHERE type = ? AND account_id = ?")
	checkError(err)

	var count int

	row := db.QueryRow("SELECT COUNT(*) FROM temp WHERE type = ? AND account_id = ?", databaseLastBuyTimestamp, userconfig.AccountID)
	err = row.Scan(&count)
	checkError(err)

	if count == 0 {
		insertTempSettings.Exec(databaseLastBuyTimestamp, userconfig.AccountID, nil, int32(time.Now().Unix()), nil)
	} else {
		updateTemp.Exec(nil, int32(time.Now().Unix()), nil, databaseLastBuyTimestamp, userconfig.AccountID)
	}

	row = db.QueryRow("SELECT COUNT(*) FROM temp WHERE type = ? AND account_id = ?", databaseLastBuyPrice, userconfig.AccountID)
	err = row.Scan(&count)
	checkError(err)

	if count == 0 {
		insertTempSettings.Exec(databaseLastBuyPrice, userconfig.AccountID, nil, nil, price)
	} else {
		updateTemp.Exec(nil, nil, price, databaseLastBuyPrice, userconfig.AccountID)
	}

	row = db.QueryRow("SELECT COUNT(*) FROM temp WHERE type = ? AND account_id = ?", databaseLastBuyInvest, userconfig.AccountID)
	err = row.Scan(&count)
	checkError(err)

	if count == 0 {
		insertTempSettings.Exec(databaseLastBuyInvest, userconfig.AccountID, nil, nil, balance)
	} else {
		updateTemp.Exec(nil, nil, balance, databaseLastBuyInvest, userconfig.AccountID)
	}

	updateTemp.Exec(nil, nil, balance, databaseLastBuyInvest)

	var depositinfo depositInfo
	depositinfo.aproxbitcoinrcv = buyValue
	depositinfo.bitcoinprice = price
	depositinfo.eurodeposit = balance
	depositinfo.ordertimeout = int64(config.SleepTimeHours*60 - 5)
	sendDepositNotificationEmail(userconfig, depositinfo)

	return err
}

func withdrawBitcoin(api *krakenapi.KrakenApi, balance float64, userconfig userconfiguration) error {
	krakenWithdrawInfo, err := api.WithdrawInfo("XBT", userconfig.WithdrawAddressDesc, new(big.Float).SetFloat64(balance))

	if err != nil {
		return err
	}

	api.Withdraw("XBT", userconfig.WithdrawAddressDesc, &krakenWithdrawInfo.Limit)

	var limit, _ = krakenWithdrawInfo.Limit.Float64()
	var fee, _ = krakenWithdrawInfo.Fee.Float64()

	print(fmt.Sprintf("Withdrawing %.5f (-%.5f BTC Fee) Bitcoin to %s.", limit, fee, userconfig.WithdrawAddressDesc))

	var withdrawinfo withdrawInfo
	withdrawinfo.addressdesc = userconfig.WithdrawAddressDesc
	withdrawinfo.balance = balance
	withdrawinfo.fee = fee
	sendWithdrawNotificationEmail(userconfig, withdrawinfo)

	var timestamp int
	var invest float64
	var price float64

	db, err := sql.Open(databaseType, config.Database)
	checkError(err)
	defer db.Close()

	row := db.QueryRow("SELECT i_value FROM temp WHERE type = ? AND account_id", databaseLastBuyTimestamp, userconfig.AccountID)
	row.Scan(&timestamp)

	row = db.QueryRow("SELECT d_value FROM temp WHERE type = ? AND account_id", databaseLastBuyInvest, userconfig.AccountID)
	row.Scan(&invest)

	row = db.QueryRow("SELECT d_value FROM temp WHERE type = ? AND account_id", databaseLastBuyPrice, userconfig.AccountID)
	row.Scan(&price)

	insertBuy, err := db.Prepare("INSERT INTO invest (account_id, timestamp, invest, bitcoin, price) VALUES (?, ?, ?, ?, ?)")
	checkError(err)
	insertBuy.Exec(userconfig.AccountID, timestamp, invest, (limit - fee), price)

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

func checkError(err error) {
	if err != nil {
		log.Fatal("Error: " + err.Error())
	}
}
