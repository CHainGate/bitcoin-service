package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"math/big"

	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"github.com/CHainGate/bitcoin-service/internal/repository"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/CHainGate/bitcoin-service/openApi"
	"github.com/CHainGate/bitcoin-service/proxyClientApi"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
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
	paymentRepository repository.IPaymentRepository
}

func NewBitcoinService(
	accountRepository repository.IAccountRepository,
	paymentRepository repository.IPaymentRepository,
) (IBitcoinService, error) {
	s := &bitcoinService{accountRepository: accountRepository, paymentRepository: paymentRepository}
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
	mode, ok := enum.ParseStringToModeEnum(paymentRequest.Mode)
	if !ok {
		return nil, errors.New("wrong mode")
	}
	priceCurrency, ok := enum.ParseStringToFiatCurrencyEnum(paymentRequest.PriceCurrency)
	if !ok {
		return nil, errors.New("wrong price currency")
	}

	payAmountInBtc, err := s.getExchangeRate(paymentRequest.PriceAmount, priceCurrency)
	if err != nil {
		return nil, err
	}

	payAmountInSatoshi := s.convertBtcToSatoshi(payAmountInBtc)

	account, err := s.getFreeAccount(mode)
	if err != nil {
		return nil, err
	}

	state := model.PaymentState{
		Base:           model.Base{ID: uuid.New()},
		PayAmount:      model.NewBigInt(payAmountInSatoshi),
		AmountReceived: model.NewBigInt(big.NewInt(0)),
		StateName:      enum.Waiting,
	}

	payment := model.Payment{
		Account:               account,
		UserWallet:            paymentRequest.Wallet,
		Mode:                  mode,
		PriceAmount:           paymentRequest.PriceAmount,
		PriceCurrency:         priceCurrency,
		CurrentPaymentState:   state,
		CurrentPaymentStateId: &state.ID,
		PaymentStates:         []model.PaymentState{state},
		Confirmations:         -1,
	}

	err = s.paymentRepository.Create(&payment)
	if err != nil {
		return nil, err
	}

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

	freeAccount.Used = true
	err = s.accountRepository.Update(freeAccount)
	if err != nil {
		return nil, err
	}
	return freeAccount, nil
}

func (s *bitcoinService) getExchangeRate(priceAmount float64, priceCurrency enum.FiatCurrency) (float64, error) {
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

func (s *bitcoinService) convertBtcToSatoshi(val float64) *big.Int {
	bigVal := new(big.Float)
	bigVal.SetFloat64(val)
	balance := big.NewFloat(0).Mul(bigVal, big.NewFloat(100000000))
	final, accur := balance.Int(nil)
	if accur == big.Below {
		final.Add(final, big.NewInt(1))
	}
	return final
}
