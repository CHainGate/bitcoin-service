package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/CHainGate/bitcoin-service/proxyClientApi"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
)

func getPayAmount(priceAmount float64, priceCurrency enum.FiatCurrency) (float64, error) {
	amount := fmt.Sprintf("%g", priceAmount)
	srcCurrency := priceCurrency.String()
	dstCurrency := enum.BTC.String()
	mode := enum.Main.String()

	configuration := proxyClientApi.NewConfiguration()
	configuration.Servers[0].URL = utils.Opts.ProxyBaseUrl
	apiClient := proxyClientApi.NewAPIClient(configuration)
	resp, _, err := apiClient.ConversionApi.GetPriceConversion(context.Background()).Amount(amount).SrcCurrency(srcCurrency).DstCurrency(dstCurrency).Mode(mode).Execute()
	if err != nil {
		return 0, err
	}

	return *resp.Price, nil
}

func getNetParams(client *rpcclient.Client) (*chaincfg.Params, error) {
	info, err := client.GetBlockChainInfo()
	if err != nil {
		return nil, err
	}

	switch info.Chain {
	case "regtest":
		return &chaincfg.RegressionNetParams, nil
	case "test":
		return &chaincfg.TestNet3Params, nil
	case "main":
		return &chaincfg.MainNetParams, nil
	default:
		return nil, errors.New("net not available")
	}
}

func convertBtcToSatoshi(val float64) (*big.Int, error) {
	amount, err := btcutil.NewAmount(val)
	if err != nil {
		return nil, err
	}

	satoshiString := amount.Format(btcutil.AmountSatoshi)
	//satoshiString -> 1000 Satoshis -> split by space
	satoshi := strings.Split(satoshiString, " ")
	result := new(big.Int)
	result, ok := result.SetString(satoshi[0], 10)
	if !ok {
		return nil, err
	}
	return result, nil
}

func CreateBitcoinTestClient() (*rpcclient.Client, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         utils.Opts.BitcoinTestHost,
		User:         utils.Opts.BitcoinTestUser,
		Pass:         utils.Opts.BitcoinTestPass,
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}
	//defer client.Shutdown()
	return client, nil
}

func CreateBitcoinMainClient() (*rpcclient.Client, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         utils.Opts.BitcoinMainHost,
		User:         utils.Opts.BitcoinMainUser,
		Pass:         utils.Opts.BitcoinMainPass,
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}
	//defer client.Shutdown()
	return client, nil
}

// only forward 99%. 1% chaingate fee
func calculateForwardAmount(amount *big.Int) *big.Int {
	mul := big.NewInt(0).Mul(amount, big.NewInt(99))
	return mul.Div(mul, big.NewInt(100))
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
