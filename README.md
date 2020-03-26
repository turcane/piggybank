# Kraken PiggyBank
This tool helps to create a PiggyBank with your Kraken Account
## How to use
1. Create an API Key on Kraken with following permissions:
    - Funds - Query Funds
    - Funds - Withdraw Funds
    - Orders & Trades - Query Open Orders & Trades
    - Orders & Trades - Query Closed Orders & Trades
    - Orders & Trades - Modify Orders
    - Orders & Trades - Cancel/Close Orders
2. Add API Key from Kraken to config.json
3. Add Private Key from Kraken to config.json
4. Create a Withdrawn Address on Kraken
5. Add the description (not the address itself) of your withdrawal address to config.json
6. Run piggybank

## How to build

1. Clode this repository
2. go build -o dist/piggybank piggybank.go
3. Alternatively you can use make (Makefile is optimized for using on macOS)
    - make => Builds PiggyBank
    - make serve => Builds and runs PiggyBank
    - make release => Creates releases of PiggyBank for different architectures