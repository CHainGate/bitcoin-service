package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"github.com/CHainGate/bitcoin-service/internal/repository"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/CHainGate/bitcoin-service/openApi"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"math/big"
)

type IBitcoinService interface {
	createNewPayment(paymentRequest openApi.PaymentRequestDto) (*model.Payment, error)
	getReceivedByAddress(account string, mode enum.Mode)
	sendToAddress(account string, mode enum.Mode) error
}

type bitcoinService struct {
	testClient        *rpcclient.Client
	mainClient        *rpcclient.Client
	accountRepository repository.IAccountRepository
}

func NewBitcoinService(accountRepository repository.IAccountRepository) (IBitcoinService, error) {
	s := &bitcoinService{accountRepository: accountRepository}
	testClient, err := s.createBitcoinTestClient()
	if err != nil {
		return nil, err
	}
	/*	mainClient, err := s.createBitcoinMainClient()
		if err != nil {
			return nil, err
		}*/
	s.testClient = testClient
	//s.mainClient = mainClient
	return s, nil
}

func (s *bitcoinService) createNewPayment(paymentRequest openApi.PaymentRequestDto) (*model.Payment, error) {
	// get exchange rate
	s.getExchangeRate()

	mode, ok := enum.ParseStringToModeEnum(paymentRequest.Mode)
	if !ok {
		return nil, errors.New("wrong mode")
	}
	priceCurrency, ok := enum.ParseStringToFiatCurrencyEnum(paymentRequest.PriceCurrency)
	if !ok {
		return nil, errors.New("wrong price currency")
	}

	account, err := s.getFreeAccount(mode)
	if err != nil {
		return nil, err
	}

	state := model.PaymentState{
		PayAmount:      model.NewBigInt(big.NewInt(0)), // TODO: add real amount
		AmountReceived: model.NewBigInt(big.NewInt(0)),
		StateName:      enum.Waiting,
	}

	payment := model.Payment{
		Account:             account,
		UserWallet:          paymentRequest.Wallet,
		Mode:                mode,
		PriceAmount:         paymentRequest.PriceAmount,
		PriceCurrency:       priceCurrency,
		CurrentPaymentState: state,
		PaymentStates:       []model.PaymentState{state},
		Confirmations:       -1,
	}
	account.Used = true
	account.Payments = append(account.Payments, payment)

	return &payment, nil
}

func (s *bitcoinService) getReceivedByAddress(account string, mode enum.Mode) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		//return nil, err
	}

	address, err := btcutil.DecodeAddress("tb1qjfkmn6ytt3xdy4cx562qwa2cuzx5r228zxve9r", &chaincfg.TestNet3Params)
	if err != nil {
		//return btcutil.Amount(0)
	}
	amount, err := client.GetReceivedByAddress(address)
	fmt.Println(amount)
	//return amount
}

func (s *bitcoinService) sendToAddress(account string, mode enum.Mode) error {
	client, err := s.getClientByMode(mode)
	if err != nil {
		//return nil, err
	}

	address, err := json.Marshal("mrdhfyD6AKtfKeok78NPoAJrudCdzAd8G6")
	if err != nil {
		fmt.Println(err)
	}
	amount, err := json.Marshal(btcutil.Amount(5234).ToBTC())
	if err != nil {
		fmt.Println(err)
	}
	comment, err := json.Marshal("")
	if err != nil {
		fmt.Println(err)
	}
	commentTo, err := json.Marshal("")
	if err != nil {
		fmt.Println(err)
	}
	subtractfeefromamount, err := json.Marshal(btcjson.Bool(true))
	if err != nil {
		fmt.Println(err)
	}

	result, err := client.RawRequest("sendtoaddress", []json.RawMessage{address, amount, comment, commentTo, subtractfeefromamount})
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(result.MarshalJSON())
	return nil
}

func (s *bitcoinService) createBitcoinTestClient() (*rpcclient.Client, error) {
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

func (s *bitcoinService) createBitcoinMainClient() (*rpcclient.Client, error) {
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

func (s *bitcoinService) getFreeAccount(mode enum.Mode) (*model.Account, error) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		return nil, err
	}

	freeAccount, err := s.accountRepository.FindUnused()
	if err != nil {
		return nil, err
	}

	if freeAccount == nil {
		newAddress, err := client.GetNewAddress("")
		if err != nil {
			return nil, err
		}
		newAccount := &model.Account{
			Address:   newAddress.String(),
			Used:      true,
			Remainder: model.NewBigInt(big.NewInt(0)),
		}
		err = s.accountRepository.Create(newAccount)
		if err != nil {
			return nil, err
		}
		return newAccount, nil
	}

	return freeAccount, nil
}

func (s *bitcoinService) getExchangeRate() error {
	return nil
}

func (s *bitcoinService) getClientByMode(mode enum.Mode) (*rpcclient.Client, error) {
	switch mode {
	case enum.Test:
		return s.testClient, nil
	case enum.Main:
		return s.mainClient, nil
	default:
		return nil, errors.New("mode not implemented")
	}
}
