package service

import (
	"errors"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/google/uuid"
	"log"
	"math/big"

	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"github.com/CHainGate/bitcoin-service/internal/repository"
	"github.com/CHainGate/bitcoin-service/openApi"
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

	enough, err := s.isPayAmountEnough(mode, payAmountInSatoshi)
	if err != nil {
		return nil, err
	}

	if !enough {
		return nil, errors.New("Pay amount is to low ")
	}

	account, err := s.getFreeAccount(mode)
	if err != nil {
		return nil, err
	}

	state := model.PaymentState{
		Base:           model.Base{ID: uuid.New()},
		PayAmount:      model.NewBigInt(payAmountInSatoshi),
		AmountReceived: model.NewBigInt(big.NewInt(0)),
		StateID:        enum.Waiting,
	}

	payment := model.Payment{
		Account:               account,
		MerchantWallet:        paymentRequest.Wallet,
		Mode:                  mode,
		PriceAmount:           paymentRequest.PriceAmount,
		PriceCurrency:         priceCurrency,
		CurrentPaymentState:   state,
		CurrentPaymentStateId: &state.ID,
		PaymentStates:         []model.PaymentState{state},
	}

	err = s.paymentRepository.Create(&payment)
	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func (s *bitcoinService) HandleWalletNotify(txId string, mode enum.Mode) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		log.Println(err)
		return
	}

	transaction, err := getTransaction(client, txId)
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

	if currentPayment.ReceivedConfirmations != nil && *currentPayment.ReceivedConfirmations >= 0 && currentPayment.CurrentPaymentState.StateID == enum.Paid {
		log.Println("payment already handled")
		return
	}

	amountReceived, err := s.getUnspentByAddress(address, 0, mode)
	if err != nil {
		log.Println(err)
		return
	}

	currentPayment.ReceivedConfirmations = &transaction.Confirmations
	amountReceived.Sub(amountReceived, &currentPayment.Account.Remainder.Int)
	var diff = currentPayment.CurrentPaymentState.PayAmount.Cmp(amountReceived)

	newState := model.PaymentState{
		Base:           model.Base{ID: uuid.New()},
		PayAmount:      currentPayment.CurrentPaymentState.PayAmount,
		AmountReceived: model.NewBigInt(amountReceived),
		PaymentID:      currentPayment.CurrentPaymentState.PaymentID,
	}

	if diff > 0 {
		newState.StateID = enum.PartiallyPaid
	} else {
		newState.StateID = enum.Paid
	}

	currentPayment.PaymentStates = append(currentPayment.PaymentStates, newState)

	currentPayment.CurrentPaymentState = newState
	currentPayment.CurrentPaymentStateId = &newState.ID

	err = sendNotificationToBackend(currentPayment.ID.String(),
		currentPayment.CurrentPaymentState.PayAmount.String(),
		currentPayment.CurrentPaymentState.AmountReceived.String(),
		currentPayment.CurrentPaymentState.StateID.String(),
		currentPayment.ForwardingTransactionHash)

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

func (s *bitcoinService) HandleBlockNotify(blockHash string, mode enum.Mode) {
	s.handlePaidPayments(mode)
	s.handleConfirmedPayments(mode)
	s.handleForwardedTransactions(mode)

	// TODO: if this runes parallel with the other jobs we need to be careful
	// maybe open transactions where time.now() - created <= 0
	s.handleExpiredTransactions(mode)
}

func (s *bitcoinService) handlePaidPayments(mode enum.Mode) {
	payments, err := s.paymentRepository.FindPaidPaymentsByMode(mode)
	if err != nil {
		log.Println(err)
		return
	}

	for _, payment := range payments {
		amountReceived, err := s.getUnspentByAddress(payment.Account.Address, utils.Opts.MinimumConfirmations, mode)
		if err != nil {
			log.Println(err)
			return
		}

		amountReceived.Sub(amountReceived, &payment.Account.Remainder.Int)
		var diff = payment.CurrentPaymentState.PayAmount.Cmp(amountReceived)

		if diff > 0 {
			return // not enough
		}

		confirmedState := model.PaymentState{
			Base:           model.Base{ID: uuid.New()},
			PayAmount:      payment.CurrentPaymentState.PayAmount,
			AmountReceived: model.NewBigInt(amountReceived),
			StateID:        enum.Confirmed,
		}

		receivedConfirmations := int64(utils.Opts.MinimumConfirmations)
		payment.ReceivedConfirmations = &receivedConfirmations
		payment.CurrentPaymentStateId = &confirmedState.ID
		payment.CurrentPaymentState = confirmedState
		payment.PaymentStates = append(payment.PaymentStates, confirmedState)

		err = sendNotificationToBackend(payment.ID.String(),
			payment.CurrentPaymentState.PayAmount.String(),
			payment.CurrentPaymentState.AmountReceived.String(),
			payment.CurrentPaymentState.StateID.String(),
			payment.ForwardingTransactionHash)

		if err != nil {
			log.Println(err)
			return
		}

		err = s.paymentRepository.Update(&payment)
		if err != nil {
			log.Println(err)
			return
		}

		forwardAmount := calculateForwardAmount(&payment.CurrentPaymentState.PayAmount.Int)
		//TODO: if multiple blocknotify at the same time we send multiple times, but should in reality never happen
		txHash, err := s.createTransaction(payment.Account.Address, payment.MerchantWallet, forwardAmount, mode)
		if err != nil {
			log.Println(err)
			return
		}
		var conf int64 = 0
		payment.ForwardingTransactionHash = &txHash
		payment.ForwardingConfirmations = &conf

		err = s.paymentRepository.Update(&payment)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func (s *bitcoinService) handleConfirmedPayments(mode enum.Mode) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		log.Println(err)
		return
	}
	payments, err := s.paymentRepository.FindConfirmedPaymentsByMode(mode)
	if err != nil {
		log.Println(err)
		return
	}

	for _, payment := range payments {
		amount, err := s.getUnspentByAddress(payment.Account.Address, utils.Opts.MinimumConfirmations, mode)
		if err != nil {
			log.Println(err)
			return
		}

		// sending failed
		if payment.ForwardingTransactionHash == nil && amount.Cmp(&payment.CurrentPaymentState.PayAmount.Int) >= 0 {
			forwardAmount := calculateForwardAmount(&payment.CurrentPaymentState.PayAmount.Int)
			//TODO: if multiple blocknotify at the same time we send multiple times, but should in reality never happen
			txHash, err := s.createTransaction(payment.Account.Address, payment.MerchantWallet, forwardAmount, mode)
			if err != nil {
				log.Println(err)
				return
			}
			var conf int64 = 0
			payment.ForwardingTransactionHash = &txHash
			payment.ForwardingConfirmations = &conf

			err = s.paymentRepository.Update(&payment)
			if err != nil {
				log.Println(err)
				return
			}
			return
		}

		// already sent but could not save txId to db
		if payment.ForwardingTransactionHash == nil && amount.Cmp(big.NewInt(0)) == 0 {
			transactions, err := s.findMissingTransaction(payment.MerchantWallet, mode)
			if err != nil {
				log.Println(err)
				return
			}

			forwardAmount := calculateForwardAmount(&payment.CurrentPaymentState.PayAmount.Int)
			forwardAmountInBtc := btcutil.Amount(forwardAmount.Int64()).ToBTC()
			for _, tx := range transactions {
				if forwardAmountInBtc == tx.amount+tx.fee {
					payment.ForwardingTransactionHash = &tx.txId
					break
				}
			}
		}

		transaction, err := getTransaction(client, *payment.ForwardingTransactionHash)
		if err != nil {
			log.Println(err)
			return
		}

		// transaction not confirmed
		if transaction.Confirmations <= 0 {
			return
		}

		sentState := model.PaymentState{
			Base:           model.Base{ID: uuid.New()},
			PayAmount:      payment.CurrentPaymentState.PayAmount,
			AmountReceived: payment.CurrentPaymentState.AmountReceived,
			StateID:        enum.Forwarded,
		}

		payment.ForwardingConfirmations = &transaction.Confirmations
		payment.CurrentPaymentStateId = &sentState.ID
		payment.CurrentPaymentState = sentState
		payment.PaymentStates = append(payment.PaymentStates, sentState)

		err = sendNotificationToBackend(payment.ID.String(),
			payment.CurrentPaymentState.PayAmount.String(),
			payment.CurrentPaymentState.AmountReceived.String(),
			payment.CurrentPaymentState.StateID.String(),
			payment.ForwardingTransactionHash)

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
	client, err := s.getClientByMode(mode)
	if err != nil {
		log.Println(err)
		return
	}

	payments, err := s.paymentRepository.FindForwardedPaymentsByMode(mode)
	if err != nil {
		log.Println(err)
		return
	}

	for _, payment := range payments {
		transaction, err := getTransaction(client, *payment.ForwardingTransactionHash)
		if err != nil {
			log.Println(err)
			return
		}

		if transaction.Confirmations >= int64(utils.Opts.MinimumConfirmations) {
			finishState := model.PaymentState{
				Base:           model.Base{ID: uuid.New()},
				PayAmount:      payment.CurrentPaymentState.PayAmount,
				AmountReceived: payment.CurrentPaymentState.AmountReceived,
				StateID:        enum.Finished,
			}

			payment.ForwardingConfirmations = &transaction.Confirmations
			payment.CurrentPaymentStateId = &finishState.ID
			payment.CurrentPaymentState = finishState
			payment.PaymentStates = append(payment.PaymentStates, finishState)
			payment.Account.Used = false

			if payment.Account.Remainder.Cmp(big.NewInt(0)) > 0 {
				unspentAmount, err := s.getUnspentByAddress(payment.Account.Address, 0, mode)
				if err != nil {
					log.Println(err)
					return
				}
				payment.Account.Remainder = model.NewBigInt(unspentAmount)
			}

			err = sendNotificationToBackend(payment.ID.String(),
				payment.CurrentPaymentState.PayAmount.String(),
				payment.CurrentPaymentState.AmountReceived.String(),
				payment.CurrentPaymentState.StateID.String(),
				payment.ForwardingTransactionHash)

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
	payments, err := s.paymentRepository.FindExpiredPaymentsByMode(mode)
	if err != nil {
		log.Println(err)
		return
	}

	for _, payment := range payments {
		receivedAmount, err := s.getUnspentByAddress(payment.Account.Address, 0, mode)
		if err != nil {
			log.Println(err)
			return
		}

		var newState model.PaymentState

		// he has paid but we did not get the notifications
		if receivedAmount.Cmp(&payment.CurrentPaymentState.PayAmount.Int) >= 0 {
			newState = model.PaymentState{
				Base:           model.Base{ID: uuid.New()},
				PayAmount:      payment.CurrentPaymentState.PayAmount,
				AmountReceived: model.NewBigInt(receivedAmount),
				PaymentID:      payment.CurrentPaymentState.PaymentID,
				StateID:        enum.Paid,
			}
		} else {
			newState = model.PaymentState{
				Base:           model.Base{ID: uuid.New()},
				PayAmount:      payment.CurrentPaymentState.PayAmount,
				AmountReceived: payment.CurrentPaymentState.AmountReceived,
				PaymentID:      payment.CurrentPaymentState.PaymentID,
				StateID:        enum.Expired,
			}
			payment.Account.Used = false
			// if the buyer has partially_paid but the transaction is expired
			if receivedAmount.Cmp(big.NewInt(0)) > 0 {
				newRemainder := payment.Account.Remainder.Add(&payment.Account.Remainder.Int, receivedAmount)
				payment.Account.Remainder = model.NewBigInt(newRemainder)
			}
		}

		payment.CurrentPaymentStateId = &newState.ID
		payment.CurrentPaymentState = newState
		payment.PaymentStates = append(payment.PaymentStates, newState)

		err = sendNotificationToBackend(payment.ID.String(),
			payment.CurrentPaymentState.PayAmount.String(),
			payment.CurrentPaymentState.AmountReceived.String(),
			payment.CurrentPaymentState.StateID.String(),
			payment.ForwardingTransactionHash)

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

func (s *bitcoinService) createTransaction(fromAddress string, toAddress string, amount *big.Int, mode enum.Mode) (string, error) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		return "", err
	}

	rawTransaction, err := createRawTransaction(client, fromAddress, toAddress, amount)
	if err != nil {
		return "", err
	}

	fundedTransaction, err := fundTransaction(client, rawTransaction, mode)
	if err != nil {
		return "", err
	}

	txHash, err := signTransaction(client, fundedTransaction, mode)
	if err != nil {
		return "", err
	}
	return txHash.String(), nil
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
	freeAccount, err := s.accountRepository.FindUnusedByMode(mode)
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
			Mode:    mode,
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

func (s *bitcoinService) findMissingTransaction(merchantWallet string, mode enum.Mode) ([]recoverSentTransactionResult, error) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		return nil, err
	}

	transactions, err := client.ListTransactions("*")
	if err != nil {
		return nil, err
	}

	txIds, err := s.paymentRepository.FindAllOutgoingTransactionIdsByMerchantWalletAndMode(merchantWallet, mode)
	if err != nil {
		return nil, err
	}

	var results []recoverSentTransactionResult
	for _, transaction := range transactions {
		if transaction.Category == "send" && transaction.Address == merchantWallet {
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

// the pay amount is heigh enough if payAmount > 2 * txFee
// and changeAmount > changeCost
func (s *bitcoinService) isPayAmountEnough(mode enum.Mode, payAmount *big.Int) (bool, error) {
	client, err := s.getClientByMode(mode)
	if err != nil {
		return false, err
	}

	feeRate, err := getFeeRate(client)
	if err != nil {
		return false, err
	}

	// tx size = (input count * 68.5) + (output coin * 31) + 10
	// tx size = 141
	const txSize = 141
	txFee, err := getFee(feeRate, txSize)
	if err != nil {
		return false, err
	}

	// changeFee = feeRate * changeOutputSize / 1000
	// costOfChange = (discardFee * changeSpendSize / 1000) + changeFee
	// we set discardFee and dustRelayFee (for dust transactions) to 0
	// this means our change need only to be higher dan changeFee
	const changeOutputSize = 32 //change address is 32 byte large. https://en.bitcoin.it/wiki/Bech32
	changeFee, err := getFee(feeRate, changeOutputSize)
	if err != nil {
		return false, err
	}

	minPayAmount := txFee.Mul(txFee, big.NewInt(2))
	forwardAmount := calculateForwardAmount(payAmount)
	changeAmount := big.NewInt(0).Sub(payAmount, forwardAmount)

	if payAmount.Cmp(minPayAmount) > 0 && changeAmount.Cmp(changeFee) > 0 {
		return true, nil
	}

	return false, nil
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
