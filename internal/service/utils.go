package service

import (
	"context"
	"fmt"
	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/CHainGate/bitcoin-service/proxyClientApi"
	"github.com/btcsuite/btcd/rpcclient"
	"math/big"
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

func convertBtcToSatoshi(val float64) *big.Int {
	bigVal := new(big.Float)
	bigVal.SetFloat64(val)
	balance := big.NewFloat(0).Mul(bigVal, big.NewFloat(100000000))
	final, accur := balance.Int(nil)
	if accur == big.Below {
		final.Add(final, big.NewInt(1))
	}
	return final
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
