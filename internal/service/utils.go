package service

import (
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/btcsuite/btcd/rpcclient"
)

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
