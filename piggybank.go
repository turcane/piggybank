package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math/big"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
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
	UserConfigs     []userconfiguration
}

type userconfiguration struct {
	AccountDescription      string
	APIKey                  string
	PrivateKey              string
	WithdrawAddressDesc     string
	MinEURWithdrawBalance   float64
	MinBTCWithdrawBalance   float64
	SendNotificationEmail   bool
	NotficationEmailAddress string
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

func main() {
	setupConfig()
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
	print("Welcome to Kraken PiggyBank v1.3")

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
	headers["To"] = userconfig.NotficationEmailAddress
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
	if err = smtpClient.Rcpt(userconfig.NotficationEmailAddress); err != nil {
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
	print(fmt.Sprintf("Notification Email send to %s for Account \"%s\".", userconfig.NotficationEmailAddress, userconfig.AccountDescription))
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

	var depositinfo depositInfo
	depositinfo.aproxbitcoinrcv = buyValue
	depositinfo.bitcoinprice = price
	depositinfo.eurodeposit = balance
	depositinfo.ordertimeout = int64(config.SleepTimeHours*60 - 5)
	sendDepositNotificationEmail(userconfig, depositinfo)

	return err
}

func withdrawBitcoin(api *krakenapi.KrakenApi, balance float64, userconfig userconfiguration) error {
	krakenWithdrawInfo, err := api.WithdrawInfo("XBT", userconfig.AccountDescription, new(big.Float).SetFloat64(balance))
	if err != nil {
		print("Could not receive Withdrawal Information: " + err.Error())
	}
	api.Withdraw("XBT", userconfig.WithdrawAddressDesc, &krakenWithdrawInfo.Limit)

	var limit, _ = krakenWithdrawInfo.Limit.Float64()
	var fee, _ = krakenWithdrawInfo.Fee.Float64()

	print(fmt.Sprintf("Withdrawing %.5f (- %.5f BTC Fee) Bitcoin to %s.", limit, fee, userconfig.WithdrawAddressDesc))

	var withdrawinfo withdrawInfo
	withdrawinfo.addressdesc = userconfig.WithdrawAddressDesc
	withdrawinfo.balance = balance
	withdrawinfo.fee, _ = krakenWithdrawInfo.Fee.Float64()
	sendWithdrawNotificationEmail(userconfig, withdrawinfo)

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
