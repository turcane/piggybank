# Kraken PiggyBank
This tool helps to create a PiggyBank with your Kraken Account. It checks every hour for new fiat funds (currently EUR only) and uses all funds to buy bitcoin. If there are bitcoin funds it will automatically withdraw to a defined bitcoin address.
## Roadmap
- [ ] Support for more Fiat Currencies
- [ ] Support for more Crypto Currencies
- [ ] Option to configure Sleep Time
- [ ] Option to deactivate withdrawal
- [ ] Notification when order was created or withdraw was done
## How to use
1. Create an API Key on Kraken with following permissions:
    - Funds - Query Funds
    - Funds - Withdraw Funds
    - Orders & Trades - Query Open Orders & Trades
    - Orders & Trades - Query Closed Orders & Trades
    - Orders & Trades - Modify Orders
    - Orders & Trades - Cancel/Close Orders
2. Add API Key from Kraken to **config.json**
3. Add Private Key from Kraken to **config.json**
4. Create a Withdraw Address on Kraken
5. Add the description (not the address itself) of your withdrawal address to **config.json**
6. Run piggybank

## How to build

1. Clone this repository
2. Install go dependencies
    - _go get github.com/beldur/kraken-go-api-client_
3. _go build -o dist/piggybank piggybank.go_
4. Alternatively you can use _make_ (Makefile is optimized for using on macOS)
    - _make_ => Builds PiggyBank
    - _make serve_ => Builds and runs PiggyBank
    - _make release_ => Creates releases of PiggyBank for different architectures
    - _make clean_ => Clean up

## Donate
Bitcoin: bc1qzuf0felpylurg69jwj20qkwgg7uy7d5kegvmkv