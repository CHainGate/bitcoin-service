package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/CHainGate/bitcoin-service/backendClientApi"
	"github.com/CHainGate/bitcoin-service/internal/utils"
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

	payAmountInSatoshi, err := convertBtcToSatoshi(payAmountInBtc)
	if err != nil {
		return nil, err
	}

	account, err := s.getFreeAccount(mode)
	if err != nil {
		log.Println(err.Error())
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
		log.Println(err)
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
		log.Println(err)
		return
	}

	if currentPayment.Confirmations >= 0 && currentPayment.CurrentPaymentState.StateName == enum.Paid {
		log.Println("payment already handled")
		return
	}

	amountReceived, err := s.getUnspentByAddress(address, 0, mode)
	if err != nil {
		log.Println(err)
		return
	}

	currentPayment.Confirmations = 0
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

	err = sendNotificationToBackend(currentPayment.ID.String(),
		currentPayment.CurrentPaymentState.PayAmount.String(),
		currentPayment.CurrentPaymentState.AmountReceived.String(),
		currentPayment.CurrentPaymentState.StateName.String())

	if err != nil {
		log.Println(err)
		return
	}

	err = s.paymentRepository.Update(currentPayment)
	if err != nil {
		log.Println(err)
		return
	}
}

func sendNotificationToBackend(paymentId string, payAmount string, actuallyPaid string, paymentState string) error {
	paymentUpdateDto := *backendClientApi.NewPaymentUpdateDto(paymentId, payAmount, enum.BTC.String(), actuallyPaid, paymentState)
	configuration := backendClientApi.NewConfiguration()
	configuration.Servers[0].URL = utils.Opts.BackendBaseUrl
	apiClient := backendClientApi.NewAPIClient(configuration)
	_, err := apiClient.PaymentUpdateApi.UpdatePayment(context.Background()).PaymentUpdateDto(paymentUpdateDto).Execute()
	if err != nil {
		return err
	}
	return nil
}

func (s *bitcoinService) HandleBlockNotify(blockHash string, mode enum.Mode) {
	s.handlePaidPayments(mode)
	s.handleConfirmedPayments(mode)
	s.handleForwardedTransactions(mode)

	// TODO: if this runes parallel with the other jobs we need to be careful
	// maybe open transactions where time.now() - created <= 0
	s.handleExpiredTransactions(mode)
}

func (s *bitcoinService) handlePaidPayments(mode enum.Mode) {
	payments, err := s.paymentRepository.FindPaidPayments()
	if err != nil {
		log.Println(err)
		return
	}

	for _, payment := range payments {
		amountReceived, err := s.getUnspentByAddress(payment.Account.Address, 6, mode)
		if err != nil {
			log.Println(err)
			return
		}

		var diff = payment.CurrentPaymentState.PayAmount.Cmp(amountReceived)

		if diff > 0 {
			return // not enough
		}

		confirmedState := model.PaymentState{
			Base:                     model.Base{ID: uuid.New()},
			PayAmount:                payment.CurrentPaymentState.PayAmount,
			AmountReceived:           model.NewBigInt(amountReceived),
			StateName:                enum.Confirmed,
			TransactionConfirmations: -1, //TODO: should be nullable, only state "sent" and "finished" have TransactionConfirmations
		}

		payment.Confirmations = 6
		payment.CurrentPaymentStateId = &confirmedState.ID
		payment.CurrentPaymentState = confirmedState
		payment.PaymentStates = append(payment.PaymentStates, confirmedState)

		err = sendNotificationToBackend(payment.ID.String(),
			payment.CurrentPaymentState.PayAmount.String(),
			payment.CurrentPaymentState.AmountReceived.String(),
			payment.CurrentPaymentState.StateName.String())

		if err != nil {
			log.Println(err)
			return
		}

		err = s.paymentRepository.Update(&payment)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func (s *bitcoinService) handleConfirmedPayments(mode enum.Mode) {
	payments, err := s.paymentRepository.FindConfirmedPayments()
	if err != nil {
		log.Println(err)
		return
	}

	for _, payment := range payments {
		amount, err := s.getUnspentByAddress(payment.Account.Address, 6, mode)
		if err != nil {
			log.Println(err)
			return
		}

		// should in theorie never happen
		if payment.CurrentPaymentState.PayAmount.Cmp(amount) > 0 {
			log.Println(err)
			return
		}

		var txId string
		if amount.Cmp(big.NewInt(0)) == 0 { // already sent
			transactions, err := s.findMissingTransaction(payment.UserWallet, mode)
			if err != nil {
				log.Println(err)
				return
			}

			amount = &payment.CurrentPaymentState.PayAmount.Int //set amount to previous payAmount because we already sent it
			forwardAmount := calculateForwardAmount(&payment.CurrentPaymentState.PayAmount.Int)
			forwardAmountInBtc := btcutil.Amount(forwardAmount.Int64()).ToBTC()
			for _, tx := range transactions {
				if forwardAmountInBtc == tx.amount+tx.fee {
					txId = tx.txId
					break
				}
			}
		} else {
			forwardAmount := calculateForwardAmount(&payment.CurrentPaymentState.PayAmount.Int)
			//TODO: if multiple blocknotify at the same time we send multiple times, but should in reality never happen
			txId, err = s.sendToAddress(payment.UserWallet, forwardAmount, mode)
			if err != nil {
				log.Println(err)
				return
			}
		}

		sentState := model.PaymentState{
			Base:                     model.Base{ID: uuid.New()},
			PayAmount:                payment.CurrentPaymentState.PayAmount,
			AmountReceived:           model.NewBigInt(amount),
			StateName:                enum.Forwarded,
			TransactionID:            txId,
			TransactionConfirmations: 0,
		}

		payment.CurrentPaymentStateId = &sentState.ID
		payment.CurrentPaymentState = sentState
		payment.PaymentStates = append(payment.PaymentStates, sentState)

		err = sendNotificationToBackend(payment.ID.String(),
			payment.CurrentPaymentState.PayAmount.String(),
			payment.CurrentPaymentState.AmountReceived.String(),
			payment.CurrentPaymentState.StateName.String())

		if err != nil {
			log.Println(err)
			return
		}

		err = s.paymentRepository.Update(&payment)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func (s *bitcoinService) handleForwardedTransactions(mode enum.Mode) {
	payments, err := s.paymentRepository.FindForwardedPayments()
	if err != nil {
		log.Println(err)
		return
	}

	for _, payment := range payments {
		transaction, err := s.getTransaction(payment.CurrentPaymentState.TransactionID, mode)
		if err != nil {
			log.Println(err)
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
			payment.Account.Used = false

			err = sendNotificationToBackend(payment.ID.String(),
				payment.CurrentPaymentState.PayAmount.String(),
				payment.CurrentPaymentState.AmountReceived.String(),
				payment.CurrentPaymentState.StateName.String())

			if err != nil {
				log.Println(err)
				return
			}

			err = s.paymentRepository.Update(&payment)
			if err != nil {
				log.Println(err)
				return
			}
			err = s.accountRepository.Update(payment.Account)
			if err != nil {
				log.Println(err)
				return
			}
		}
	}
}

func (s *bitcoinService) handleExpiredTransactions(mode enum.Mode) {
	payments, err := s.paymentRepository.FindExpiredPayments()
	if err != nil {
		log.Println(err)
		return
	}

	for _, payment := range payments {
		amount, err := s.getUnspentByAddress(payment.Account.Address, 0, mode)
		if err != nil {
			log.Println(err)
			return
		}

		var newState model.PaymentState

		// he has paid but we did not get the notifications
		if amount.Cmp(&payment.CurrentPaymentState.PayAmount.Int) >= 0 {
			newState = model.PaymentState{
				Base:           model.Base{ID: uuid.New()},
				PayAmount:      payment.CurrentPaymentState.PayAmount,
				AmountReceived: model.NewBigInt(amount),
				PaymentID:      payment.CurrentPaymentState.PaymentID,
				StateName:      enum.Paid,
			}
		} else {
			newState = model.PaymentState{
				Base:           model.Base{ID: uuid.New()},
				PayAmount:      payment.CurrentPaymentState.PayAmount,
				AmountReceived: payment.CurrentPaymentState.AmountReceived,
				PaymentID:      payment.CurrentPaymentState.PaymentID,
				StateName:      enum.Expired,
			}
		}

		payment.CurrentPaymentStateId = &newState.ID
		payment.CurrentPaymentState = newState
		payment.PaymentStates = append(payment.PaymentStates, newState)
		payment.Account.Used = false

		err = sendNotificationToBackend(payment.ID.String(),
			payment.CurrentPaymentState.PayAmount.String(),
			payment.CurrentPaymentState.AmountReceived.String(),
			payment.CurrentPaymentState.StateName.String())

		if err != nil {
			log.Println(err)
			return
		}

		err = s.paymentRepository.Update(&payment)
		if err != nil {
			log.Println(err)
			return
		}

		err = s.accountRepository.Update(payment.Account)
		if err != nil {
			log.Println(err)
			return
		}
	}
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

	passphrase, err := s.getWalletPassphraseByMode(mode)
	if err != nil {
		return "", err
	}
	err = client.WalletPassphrase(passphrase, 60)
	if err != nil {
		return "", err
	}
	result, err := client.RawRequest("sendtoaddress", []json.RawMessage{addressAsJson, amountAsJson, comment, commentTo, subtractFeeFromAmount})
	if err != nil {
		return "", err
	}
	err = client.WalletLock()
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

	params, err := getNetParams(client)
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

	amount := 0.0
	for _, unspent := range unspentList {
		amount = amount + unspent.Amount
	}

	return convertBtcToSatoshi(amount)
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
			Address: newAddress.String(),
			Used:    true,
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

type recoverSentTransactionResult struct {
	txId   string
	amount float64
	fee    float64
}

func (s *bitcoinService) findMissingTransaction(userWallet string, mode enum.Mode) ([]recoverSentTransactionResult, error) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		return nil, err
	}

	transactions, err := client.ListTransactions("*")
	if err != nil {
		return nil, err
	}

	txIds, err := s.paymentRepository.FindAllOutgoingTransactionIdsByUserWallet(userWallet)
	if err != nil {
		return nil, err
	}

	var results []recoverSentTransactionResult
	for _, transaction := range transactions {
		if transaction.Category == "send" && transaction.Address == userWallet {
			if !contains(txIds, transaction.TxID) {
				result := recoverSentTransactionResult{
					txId:   transaction.TxID,
					amount: transaction.Amount,
					fee:    *transaction.Fee,
				}
				results = append(results, result)
			}
		}
	}
	return results, nil
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

func (s *bitcoinService) getWalletPassphraseByMode(mode enum.Mode) (string, error) {
	switch mode {
	case enum.Test:
		return utils.Opts.TestWalletPassphrase, nil
	case enum.Main:
		return utils.Opts.MainWalletPassphrase, nil
	default:
		return "", errors.New("mode not implemented")
	}
}
