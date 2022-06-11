package service

import (
	"errors"
	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"math/big"
)

func getTransaction(client *rpcclient.Client, txId string) (*btcjson.GetTransactionResult, error) {
	hash, err := chainhash.NewHashFromStr(txId)
	if err != nil {
		return nil, err
	}
	transaction, err := client.GetTransaction(hash)
	if err != nil {
		return nil, err
	}
	return transaction, nil
}

func createRawTransaction(client *rpcclient.Client, fromAddress string, toAddress string, amount *big.Int) (*wire.MsgTx, error) {
	params, err := getNetParams(client)
	if err != nil {
		return nil, err
	}

	decodedFromAddress, err := btcutil.DecodeAddress(fromAddress, params)
	if err != nil {
		return nil, err
	}

	unspentList, err := client.ListUnspentMinMaxAddresses(utils.Opts.MinimumConfirmations, 9999999, []btcutil.Address{decodedFromAddress})
	if err != nil {
		return nil, err
	}

	var inputs []btcjson.TransactionInput
	for _, unspent := range unspentList {
		input := btcjson.TransactionInput{
			Txid: unspent.TxID,
			Vout: unspent.Vout,
		}
		inputs = append(inputs, input)
	}

	decodedToAddress, err := btcutil.DecodeAddress(toAddress, params)
	payAmount := btcutil.Amount(amount.Int64())
	amounts := map[btcutil.Address]btcutil.Amount{decodedToAddress: payAmount}

	rawTransaction, err := client.CreateRawTransaction(inputs, amounts, nil)
	if err != nil {
		return nil, err
	}
	return rawTransaction, nil
}

func fundTransaction(client *rpcclient.Client, rawTransaction *wire.MsgTx, mode enum.Mode) (*btcjson.FundRawTransactionResult, error) {
	feeRate, err := getFeeRate(client)
	if err != nil {
		return nil, err
	}

	var changeAddress string
	replaceable := true
	changePosition := 1
	if mode == enum.Test {
		changeAddress = utils.Opts.TestChangeAddress
	} else {
		changeAddress = utils.Opts.MainChangeAddress
	}

	opts := btcjson.FundRawTransactionOpts{
		ChangeAddress:          &changeAddress,
		FeeRate:                feeRate,
		Replaceable:            &replaceable,
		ChangePosition:         &changePosition,
		SubtractFeeFromOutputs: []int{0},
	}
	fundedTransaction, err := client.FundRawTransaction(rawTransaction, opts, nil)
	if err != nil {
		return nil, err
	}
	return fundedTransaction, nil
}

func signTransaction(client *rpcclient.Client, fundedTransaction *btcjson.FundRawTransactionResult, mode enum.Mode) (*chainhash.Hash, error) {
	var passphrase string
	if mode == enum.Test {
		passphrase = utils.Opts.TestWalletPassphrase
	} else {
		passphrase = utils.Opts.MainWalletPassphrase
	}

	err := client.WalletPassphrase(passphrase, 60)
	if err != nil {
		return nil, err
	}
	signedTransaction, areAllInputsSigned, err := client.SignRawTransactionWithWallet(fundedTransaction.Transaction)
	if err != nil {
		return nil, err
	}
	err = client.WalletLock()
	if err != nil {
		return nil, err
	}
	if !areAllInputsSigned {
		return nil, errors.New("not all inputs signed")
	}

	txHash, err := client.SendRawTransaction(signedTransaction, false)
	if err != nil {
		return nil, err
	}

	return txHash, nil
}
