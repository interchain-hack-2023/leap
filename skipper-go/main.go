package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"
	"github.com/thal0x/bn/bigint"

	"github.com/skip-mev/skipper/bindings"
	"github.com/skip-mev/skipper/bot"
	"github.com/skip-mev/skipper/feed"
	"github.com/skip-mev/skipper/services/api"
)

var (
	rootCmd             = flag.NewFlagSet("root", flag.ExitOnError)
	configFlag          = rootCmd.String("config", "./config", "path to bot configuration directory")
	contractAddressFlag = rootCmd.String("multihop", "", "address of the multihop contract")
	privateKeyFlag      = rootCmd.String("key", "", "private key to use for signing transactions (must be the deployer of the multihop contract)")
)

func logSetup(json bool, logLevel string) *logrus.Entry {
	log := logrus.NewEntry(logrus.New())
	log.Logger.SetOutput(os.Stdout)

	if json {
		log.Logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			log.Fatalf("Invalid loglevel: %s", logLevel)
		}
		log.Logger.SetLevel(lvl)
	}
	return log
}

func startCommand() {
	log := logSetup(true, "info").WithFields(logrus.Fields{
		"service": "leap/relay/api",
		"version": "0.0.1",
	})

	key, err := crypto.HexToECDSA(*privateKeyFlag)
	if err != nil {
		panic(err)
	}

	config, err := bot.LoadBotConfig(*configFlag)
	if err != nil {
		panic(err)
	}

	backrunner, err := bot.NewBot(config, common.HexToAddress(*contractAddressFlag), key)
	if err != nil {
		panic(err)
	}

	err = backrunner.Start()
	if err != nil {
		panic(err)
	}

	fmt.Println("backrunner listening for transactions...")
	apiConfig := api.RelayAPIConfig{ListenAddr: config.ApiHost, Log: log}
	api := api.NewRelayAPI(apiConfig, backrunner)
	api.StartServer()
	fmt.Println("api listening for requests...")

	txFeed := feed.NewTransactionFeed(config.CosmosRPC, config.PollMs)

	txChan := txFeed.SubscribeNewTransactions()

	for {
		tx := <-txChan

		go backrunner.OnTransaction(tx)
	}
}

func withdrawCommand() {
	config, err := bot.LoadBotConfig(*configFlag)
	if err != nil {
		panic(err)
	}

	client, err := ethclient.Dial(config.EthRPC)
	if err != nil {
		panic(err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		panic(err)
	}

	key, err := crypto.HexToECDSA(*privateKeyFlag)
	if err != nil {
		panic(err)
	}

	opts, err := bind.NewKeyedTransactorWithChainID(key, chainID)
	if err != nil {
		panic(err)
	}

	opts.GasLimit = 900000
	opts.Signer = func(a common.Address, t *types.Transaction) (*types.Transaction, error) {
		return types.SignTx(t, types.NewLondonSigner(chainID), key)
	}

	nonce, err := client.NonceAt(context.Background(), opts.From, nil)
	if err != nil {
		panic(err)
	}
	opts.Nonce = bigint.New(int64(nonce))

	contract, err := bindings.NewMultihop(common.HexToAddress(*contractAddressFlag), client)
	if err != nil {
		panic(err)
	}

	tx, err := contract.Withdraw(opts, config.BaseToken)
	if err != nil {
		panic(err)
	}

	fmt.Printf("tx: https://escan.live/tx/%s\n", tx.Hash())
}

func main() {
	rootCmd.Parse(os.Args[2:])

	if contractAddressFlag == nil || *contractAddressFlag == "" {
		panic("missing --multihop flag")
	}

	if privateKeyFlag == nil || *privateKeyFlag == "" {
		panic("missing --key flag")
	}

	switch os.Args[1] {
	case "start":
		startCommand()
	case "withdraw":
		withdrawCommand()
	default:
		fmt.Printf("expected 'start' or 'withdraw' command, got '%s'\n", os.Args[1])
		os.Exit(1)
	}
}
