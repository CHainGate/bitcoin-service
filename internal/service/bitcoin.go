package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"log"
	"math/big"
	"strings"

	"github.com/google/uuid"

	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"github.com/CHainGate/bitcoin-service/internal/repository"
	"github.com/CHainGate/bitcoin-service/openApi"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
)

type IBitcoinService interface {
	CreateNewPayment(paymentRequest openApi.PaymentRequestDto) (*model.Payment, error)
	HandleWalletNotify(txId string, mode enum.Mode)
	HandleBlockNotify(blockHash string, mode enum.Mode)
}

type bitcoinService struct {
	regtestClient     *rpcclient.Client
	testClient        *rpcclient.Client
	mainClient        *rpcclient.Client
	accountRepository repository.IAccountRepository
	paymentRepository repository.IPaymentRepository
}

func NewBitcoinService(
	accountRepository repository.IAccountRepository,
	paymentRepository repository.IPaymentRepository,
	testClient *rpcclient.Client,
	mainClient *rpcclient.Client,
) IBitcoinService {
	return &bitcoinService{
		accountRepository: accountRepository,
		paymentRepository: paymentRepository,
		testClient:        testClient,
		mainClient:        mainClient}
}

func (s *bitcoinService) CreateNewPayment(paymentRequest openApi.PaymentRequestDto) (*model.Payment, error) {
	mode, ok := enum.ParseStringToModeEnum(paymentRequest.Mode)
	if !ok {
		return nil, errors.New("wrong mode")
	}
	priceCurrency, ok := enum.ParseStringToFiatCurrencyEnum(paymentRequest.PriceCurrency)
	if !ok {
		return nil, errors.New("wrong price currency")
	}

	payAmountInBtc, err := getPayAmount(paymentRequest.PriceAmount, priceCurrency)
	if err != nil {
		return nil, err
	}

	payAmountInSatoshi := convertBtcToSatoshi(payAmountInBtc)

	account, err := s.getFreeAccount(mode)
	if err != nil {
		return nil, err
	}

	state := model.PaymentState{
		Base:                     model.Base{ID: uuid.New()},
		PayAmount:                model.NewBigInt(payAmountInSatoshi),
		AmountReceived:           model.NewBigInt(big.NewInt(0)),
		StateName:                enum.Waiting,
		TransactionConfirmations: -1, //TODO: should be nullable, only state "sent" and "finished" have TransactionConfirmations
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

func (s *bitcoinService) HandleWalletNotify(txId string, mode enum.Mode) {
	transaction, err := s.getTransaction(txId, mode)
	if err != nil {
		log.Println(err.Error())
		return
	}

	// only conf 0 is relevant (first user pay in)
	// if amount is negative it is a sending payment
	if transaction.Confirmations != 0 || transaction.Amount < 0 {
		return
	}

	address := transaction.Details[0].Address
	currentPayment, err := s.paymentRepository.FindCurrentPaymentByAddress(address)
	if err != nil {
		log.Println(err.Error())
		return
	}

	// already handled
	if currentPayment.Confirmations >= 0 {
		return
	}

	amount, err := s.getUnspentByAddress(address, 0, mode)
	if err != nil {
		log.Println(err.Error())
		return
	}

	currentPayment.Confirmations = 0
	amountReceived := amount.Sub(amount, &currentPayment.Account.Remainder.Int)
	var diff = currentPayment.CurrentPaymentState.PayAmount.Cmp(amountReceived)

	newState := model.PaymentState{
		Base:                     model.Base{ID: uuid.New()},
		PayAmount:                currentPayment.CurrentPaymentState.PayAmount,
		AmountReceived:           model.NewBigInt(amountReceived),
		PaymentID:                currentPayment.CurrentPaymentState.PaymentID,
		TransactionConfirmations: -1, //TODO: should be nullable, only state "sent" and "finished" have TransactionConfirmations
	}

	if diff > 0 {
		newState.StateName = enum.PartiallyPaid
	} else {
		newState.StateName = enum.Paid
	}

	currentPayment.PaymentStates = append(currentPayment.PaymentStates, newState)

	currentPayment.CurrentPaymentState = newState
	currentPayment.CurrentPaymentStateId = &newState.ID

	err = s.paymentRepository.Update(currentPayment)
	if err != nil {
		log.Println(err.Error())
		return
	}

	//TODO: send notification to backend
}

func (s *bitcoinService) HandleBlockNotify(blockHash string, mode enum.Mode) {

	s.checkPayments(mode)

	s.checkOutgoingTransactions(mode)

	// TODO: if this runes parallel with the other jobs we need to be careful
	// maybe open transactions where time.now() - created <= 0
	s.checkForExpiredTransactions()
}

func (s *bitcoinService) sendToAddress(address string, amount *big.Int, mode enum.Mode) (string, error) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		return "", err
	}

	addressAsJson, err := json.Marshal(address)
	if err != nil {
		return "", err
	}

	amountAsJson, err := json.Marshal(btcutil.Amount(amount.Int64()).ToBTC())
	if err != nil {
		return "", err
	}

	subtractFeeFromAmount, err := json.Marshal(btcjson.Bool(true))
	if err != nil {
		return "", err
	}

	var comment []byte
	var commentTo []byte
	result, err := client.RawRequest("sendtoaddress", []json.RawMessage{addressAsJson, amountAsJson, comment, commentTo, subtractFeeFromAmount})
	if err != nil {
		return "", err
	}

	txId, err := result.MarshalJSON()
	if err != nil {
		return "", err
	}

	cleanTxId := strings.Trim(string(txId), "\"")

	return cleanTxId, nil
}

func (s *bitcoinService) getTransaction(txId string, mode enum.Mode) (*btcjson.GetTransactionResult, error) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		return nil, err
	}
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

func (s *bitcoinService) getUnspentByAddress(address string, minConf int, mode enum.Mode) (*big.Int, error) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		return nil, err
	}

	params, err := s.getNetParams(client)
	if err != nil {
		return nil, err
	}

	decodedAddress, err := btcutil.DecodeAddress(address, params)
	if err != nil {
		return nil, err
	}

	unspentList, err := client.ListUnspentMinMaxAddresses(minConf, 9999999, []btcutil.Address{decodedAddress})
	if err != nil {
		return nil, err
	}

	var amount float64
	for _, unspent := range unspentList {
		amount = amount + unspent.Amount
	}

	return convertBtcToSatoshi(amount), nil
}

func (s *bitcoinService) getFreeAccount(mode enum.Mode) (*model.Account, error) {
	freeAccount, err := s.accountRepository.FindUnused()
	if err != nil {
		return nil, err
	}

	if freeAccount == nil {
		client, err := s.getClientByMode(mode)
		if err != nil {
			return nil, err
		}

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

func (s *bitcoinService) checkPayments(mode enum.Mode) {
	payments, err := s.paymentRepository.FindOpenPaidPayments()
	if err != nil {
		return
	}

	for _, payment := range payments {
		amount, err := s.getUnspentByAddress(payment.Account.Address, 6, mode)
		if err != nil {
			return
		}

		amountReceived := amount.Sub(amount, &payment.Account.Remainder.Int)
		var diff = payment.CurrentPaymentState.PayAmount.Cmp(amountReceived)

		if diff > 0 {
			return // not enough
		}

		confirmedState := model.PaymentState{
			Base:           model.Base{ID: uuid.New()},
			PayAmount:      payment.CurrentPaymentState.PayAmount,
			AmountReceived: model.NewBigInt(amountReceived),
			StateName:      enum.Confirmed,
		}

		payment.Confirmations = 6
		payment.CurrentPaymentStateId = &confirmedState.ID
		payment.CurrentPaymentState = confirmedState
		payment.PaymentStates = append(payment.PaymentStates, confirmedState)

		//TODO: update backend (confirmed)

		err = s.paymentRepository.Update(&payment)
		if err != nil {
			fmt.Println(err)
		}

		//TODO: if multiple blocknotify at the same time we send multiple times, but should in reality never happen
		// sendToAddress
		txId, err := s.sendToAddress(payment.UserWallet, &payment.CurrentPaymentState.PayAmount.Int, mode)
		if err != nil {
			return
		}

		sentState := model.PaymentState{
			Base:                     model.Base{ID: uuid.New()},
			PayAmount:                payment.CurrentPaymentState.PayAmount,
			AmountReceived:           model.NewBigInt(amountReceived),
			StateName:                enum.Forwarded,
			TransactionID:            txId,
			TransactionConfirmations: 0,
		}

		payment.CurrentPaymentStateId = &sentState.ID
		payment.CurrentPaymentState = sentState
		payment.PaymentStates = append(payment.PaymentStates, sentState)

		//TODO: update backend (sent)

		err = s.paymentRepository.Update(&payment)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (s *bitcoinService) checkOutgoingTransactions(mode enum.Mode) {
	payments, err := s.paymentRepository.FindOpenForwardedPayments()
	if err != nil {
		return
	}

	for _, payment := range payments {
		transaction, err := s.getTransaction(payment.CurrentPaymentState.TransactionID, mode)
		if err != nil {
			return
		}

		if transaction.Confirmations >= 6 {
			finishState := model.PaymentState{
				Base:                     model.Base{ID: uuid.New()},
				PayAmount:                payment.CurrentPaymentState.PayAmount,
				AmountReceived:           payment.CurrentPaymentState.AmountReceived,
				StateName:                enum.Finished,
				TransactionID:            payment.CurrentPaymentState.TransactionID,
				TransactionConfirmations: transaction.Confirmations,
			}
			payment.CurrentPaymentStateId = &finishState.ID
			payment.CurrentPaymentState = finishState
			payment.PaymentStates = append(payment.PaymentStates, finishState)

			//TODO: update backend (finished)

			err := s.paymentRepository.Update(&payment)
			if err != nil {
				return
			}
		}
	}
}

func (s *bitcoinService) checkForExpiredTransactions() {
	payments, err := s.paymentRepository.FindInactivePayments()
	if err != nil {
		return
	}

	for _, payment := range payments {
		failedState := model.PaymentState{
			Base:           model.Base{ID: uuid.New()},
			PayAmount:      payment.CurrentPaymentState.PayAmount,
			AmountReceived: payment.CurrentPaymentState.AmountReceived,
			PaymentID:      payment.CurrentPaymentState.PaymentID,
			StateName:      enum.Expired,
		}
		payment.CurrentPaymentStateId = &failedState.ID
		payment.CurrentPaymentState = failedState
		payment.PaymentStates = append(payment.PaymentStates, failedState)

		// todo: update backend

		s.paymentRepository.Update(&payment)
	}
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

func (s *bitcoinService) getNetParams(client *rpcclient.Client) (*chaincfg.Params, error) {
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
