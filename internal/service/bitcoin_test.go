package service

import (
	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"github.com/CHainGate/bitcoin-service/internal/repository"
	"github.com/CHainGate/bitcoin-service/openApi"
	"github.com/CHainGate/bitcoin-service/test_utils"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"github.com/ory/dockertest/v3"
	"gopkg.in/h2non/gock.v1"
	"log"
	"math/big"
	"os"
	"testing"
	"time"
)

var (
	accountRepo     repository.IAccountRepository
	paymentRepo     repository.IPaymentRepository
	chaingateClient *rpcclient.Client
	buyerClient     *rpcclient.Client
	service         IBitcoinService
	payAddress      string
)

const factor = 100000000
const payAmount = 0.003403

//const payAmount = 0.000141
const chaingateProfit = payAmount * 0.01

var testPaymentState = model.PaymentState{
	StateName:                enum.Waiting,
	PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
	AmountReceived:           model.NewBigInt(big.NewInt(0)),
	TransactionConfirmations: -1,
}

var testPayment = model.Payment{
	Account:               nil,
	UserWallet:            "",
	Mode:                  enum.Test,
	PriceAmount:           100,
	PriceCurrency:         enum.USD,
	CurrentPaymentState:   testPaymentState,
	CurrentPaymentStateId: &testPaymentState.ID,
	PaymentStates:         []model.PaymentState{testPaymentState},
	Confirmations:         -1,
}

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	//setup db
	dbRessource, r1, r2, err := testutils.DbTestSetup(pool)
	if err != nil {
		log.Fatalf("Could not setup DB: %s", err)
	}
	accountRepo = r1
	paymentRepo = r2

	//setup bitcoin node
	bitcoinSetupResult, err := testutils.BitcoinNodeTestSetup(pool)
	if err != nil {
		log.Fatalf("Could not setup Bitcoin Nodes: %s", err)
	}
	chaingateClient = bitcoinSetupResult.ChaingateClient
	buyerClient = bitcoinSetupResult.BuyerClient

	merchantAddress, err := buyerClient.GetNewAddress("")
	if err != nil {
		return
	}
	testPayment.UserWallet = merchantAddress.String()
	service = NewBitcoinService(accountRepo, paymentRepo, chaingateClient, nil)

	//Run tests
	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err = pool.Purge(bitcoinSetupResult.ChaingateRessource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
	if err = pool.Purge(bitcoinSetupResult.BuyerRessource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
	if err = pool.Purge(dbRessource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
	os.Exit(code)
}

func TestBitcoinService_CreateNewPayment(t *testing.T) {
	// Arrange
	defer gock.Off()

	gock.New("http://localhost:8001").
		Get("/api/price-conversion").
		MatchParam("amount", "100").
		MatchParam("dst_currency", "btc").
		MatchParam("mode", "main").
		MatchParam("src_currency", "usd").
		Reply(200).
		JSON(map[string]interface{}{"src_currency": "usd", "dst_currency": "btc", "price": payAmount})

	request := openApi.PaymentRequestDto{
		PriceCurrency: "usd",
		PriceAmount:   100,
		Wallet:        testPayment.UserWallet,
		Mode:          "test",
	}

	// Act
	payment, err := service.CreateNewPayment(request)
	if err != nil {
		t.Errorf("Create paymend got an error: %s", err)
	}

	// Assert
	if payment.Account.Address == "" ||
		payment.UserWallet != testPayment.UserWallet ||
		payment.Mode != testPayment.Mode ||
		payment.PriceAmount != testPayment.PriceAmount ||
		payment.PriceCurrency != testPayment.PriceCurrency ||
		payment.CurrentPaymentStateId.String() == "" ||
		payment.Confirmations != testPayment.Confirmations {
		t.Errorf("Expected address to not be empty, but got %s", payment.Account.Address)
		t.Errorf("Expected %s, but got %s", testPayment.UserWallet, payment.UserWallet)
		t.Errorf("Expected %d, but got %d", testPayment.Mode, payment.Mode)
		t.Errorf("Expected %f, but got %f", testPayment.PriceAmount, payment.PriceAmount)
		t.Errorf("Expected %d, but got %d", testPayment.PriceCurrency, payment.PriceCurrency)
		t.Errorf("Expected CurrentPaymentStateId to not be ampty, but got %s", payment.CurrentPaymentStateId.String())
		t.Errorf("Expected %d, but got %d", testPayment.Confirmations, payment.Confirmations)
	}

	payAmountCmp := payment.PaymentStates[0].PayAmount.Cmp(&testPaymentState.PayAmount.Int)
	amountReceivedCmp := payment.PaymentStates[0].AmountReceived.Cmp(&testPaymentState.AmountReceived.Int)

	if payment.PaymentStates[0].StateName != testPaymentState.StateName ||
		payment.PaymentStates[0].TransactionConfirmations != testPaymentState.TransactionConfirmations ||
		payAmountCmp != 0 ||
		amountReceivedCmp != 0 {
		t.Errorf("Expected payment state to be: %v but got %v", testPaymentState, payment.PaymentStates[0])
	}

	payAddress = payment.Account.Address
}

func TestBitcoinService_HandleWalletNotify(t *testing.T) {
	// Arrange
	defer gock.Off()

	gock.New("http://localhost:8000").
		Put("/api/internal/payment/webhook").
		MatchType("json").
		Reply(200)

	decodedAddress, err := btcutil.DecodeAddress(payAddress, &chaincfg.RegressionNetParams)
	amount, err := btcutil.NewAmount(payAmount)
	if err != nil {
		t.Errorf("%v", err)
	}
	txId, err := buyerClient.SendToAddress(decodedAddress, amount)
	if err != nil {
		t.Errorf("%v", err)
	}

	// wait for transaction to be published
	time.Sleep(10 * time.Second)

	// Act
	service.HandleWalletNotify(txId.String(), enum.Test)

	// Assert
	account, err := accountRepo.FindByAddress(payAddress)
	if err != nil {
		t.Errorf("%v", err)
	}

	cmpPayAmount := account.Payments[0].CurrentPaymentState.PayAmount.Cmp(&testPaymentState.PayAmount.Int)
	cmpAmountReceived := account.Payments[0].CurrentPaymentState.AmountReceived.Cmp(&testPaymentState.PayAmount.Int)

	if len(account.Payments) != 1 ||
		account.Payments[0].CurrentPaymentState.StateName != enum.Paid ||
		cmpPayAmount != 0 ||
		cmpAmountReceived != 0 ||
		account.Payments[0].CurrentPaymentState.TransactionConfirmations != -1 ||
		account.Payments[0].Confirmations != 0 {
		t.Errorf("Expected: %d got: %d", 1, len(account.Payments))
		t.Errorf("Expected: %d got: %d", enum.Paid, account.Payments[0].CurrentPaymentState.StateName)
		t.Errorf("Expected: %d got: %d", 0, cmpPayAmount)
		t.Errorf("Expected: %d got: %d", 0, cmpAmountReceived)
		t.Errorf("Expected: %d got: %d", -1, account.Payments[0].CurrentPaymentState.TransactionConfirmations)
		t.Errorf("Expected: %d got: %d", 0, account.Payments[0].Confirmations)
	}

	balance, err := chaingateClient.ListUnspentMin(0)
	if balance[0].Amount != payAmount {
		t.Errorf("Expected unspent amount %f, but got %f", payAmount, balance[0].Amount)
	}
}

func TestBitcoinService_HandleBlockNotify(t *testing.T) {
	// Arrange
	defer gock.Off()
	gock.New("http://localhost:8000").
		Put("/api/internal/payment/webhook").
		MatchType("json").
		Reply(200)
	gock.New("http://localhost:8000").
		Put("/api/internal/payment/webhook").
		MatchType("json").
		Reply(200)

	address, err := buyerClient.GetNewAddress("")
	if err != nil {
		return
	}
	// 6x
	var maxTries int64
	maxTries = 1000000 //default
	for i := 0; i < 6; i++ {
		_, err = buyerClient.GenerateToAddress(1, address, &maxTries)
		if err != nil {
			return
		}
	}

	// Act
	service.HandleBlockNotify("", enum.Test)

	// Assert
	account, err := accountRepo.FindByAddress(payAddress)
	if err != nil {
		t.Errorf("%v", err)
	}

	expectedStates := []model.PaymentState{
		{
			StateName:                enum.Waiting,
			PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
			AmountReceived:           model.NewBigInt(big.NewInt(0)),
			TransactionConfirmations: -1,
		},
		{
			StateName:                enum.Paid,
			PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
			AmountReceived:           model.NewBigInt(big.NewInt(payAmount * factor)),
			TransactionConfirmations: -1,
		},
		{
			StateName:                enum.Confirmed,
			PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
			AmountReceived:           model.NewBigInt(big.NewInt(payAmount * factor)),
			TransactionConfirmations: -1,
		},
		{
			StateName:                enum.Forwarded,
			PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
			AmountReceived:           model.NewBigInt(big.NewInt(payAmount * factor)),
			TransactionConfirmations: 0,
		},
	}
	for i, paymentState := range account.Payments[0].PaymentStates {
		if paymentState.StateName != expectedStates[i].StateName ||
			paymentState.PayAmount.Cmp(&expectedStates[i].PayAmount.Int) != 0 ||
			paymentState.AmountReceived.Cmp(&expectedStates[i].AmountReceived.Int) != 0 ||
			paymentState.TransactionConfirmations != expectedStates[i].TransactionConfirmations {
			t.Errorf("Expected: %v got %v", expectedStates[i].StateName, paymentState.StateName)
			t.Errorf("Expected: %s got %s", expectedStates[i].PayAmount.String(), paymentState.PayAmount.String())
			t.Errorf("Expected: %s got %s", expectedStates[i].AmountReceived.String(), paymentState.AmountReceived.String())
			t.Errorf("Expected: %d got %d", expectedStates[i].TransactionConfirmations, paymentState.TransactionConfirmations)
		}
		if paymentState.StateName == enum.Forwarded && paymentState.TransactionID == "" {
			t.Errorf("Expected transactionID not to be empty, but got %s", paymentState.TransactionID)
		}
	}

	if account.Payments[0].Confirmations != 6 {
		t.Errorf("Expected confirmations %d, but got %d", 6, account.Payments[0].Confirmations)
	}

	// wait for transaction to be published
	time.Sleep(10 * time.Second)
	//TODO: bitcoin sends all money if the difference is small
	hash, err := chainhash.NewHashFromStr(account.Payments[0].PaymentStates[3].TransactionID)
	if err != nil {
		t.Error(err)
	}
	transaction, err := chaingateClient.GetTransaction(hash)
	if err != nil {
		t.Error(err)
	}

	decodedAddress, err := btcutil.DecodeAddress(testPayment.UserWallet, &chaincfg.RegressionNetParams)
	balance, err := buyerClient.ListUnspentMinMaxAddresses(0, 999999, []btcutil.Address{decodedAddress})

	amount := big.NewFloat(balance[0].Amount)
	fee := big.NewFloat(transaction.Fee)
	//transaction.Fee is a negative number!!
	totalSent := amount.Sub(amount, fee)

	a := big.NewFloat(payAmount)
	b := big.NewFloat(0.99)
	forwardAmount := a.Mul(a, b)
	diff, _ := forwardAmount.Sub(forwardAmount, totalSent).Float64()
	if diff > 0.000000001 {
		t.Errorf("Expected unspent amount %f, but got %f", forwardAmount, balance[0].Amount)
	}

	chaingateBalance, err := chaingateClient.GetBalance("*")
	if err != nil {
		t.Error(err)
	}

	if chaingateBalance.ToBTC() != chaingateProfit {
		t.Errorf("Expected chaingate profit to be %f, but got %f", chaingateProfit, chaingateBalance.ToBTC())
	}
}

func TestBitcoinService_HandleBlockNotify2(t *testing.T) {
	// Arrange
	defer gock.Off()

	gock.New("http://localhost:8000").
		Put("/api/internal/payment/webhook").
		MatchType("json").
		Reply(200)

	address, err := buyerClient.GetNewAddress("")
	if err != nil {
		return
	}
	// 6x
	var maxTries int64
	maxTries = 1000000 //default
	for i := 0; i < 6; i++ {
		_, err := buyerClient.GenerateToAddress(1, address, &maxTries)
		if err != nil {
			return
		}
	}

	// Act
	service.HandleBlockNotify("", enum.Test)

	// Assert
	account, err := accountRepo.FindByAddress(payAddress)
	if err != nil {
		t.Errorf("%v", err)
	}

	expectedStates := []model.PaymentState{
		{
			StateName:                enum.Waiting,
			PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
			AmountReceived:           model.NewBigInt(big.NewInt(0)),
			TransactionConfirmations: -1,
		},
		{
			StateName:                enum.Paid,
			PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
			AmountReceived:           model.NewBigInt(big.NewInt(payAmount * factor)),
			TransactionConfirmations: -1,
		},
		{
			StateName:                enum.Confirmed,
			PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
			AmountReceived:           model.NewBigInt(big.NewInt(payAmount * factor)),
			TransactionConfirmations: -1,
		},
		{
			StateName:                enum.Forwarded,
			PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
			AmountReceived:           model.NewBigInt(big.NewInt(payAmount * factor)),
			TransactionConfirmations: 0,
		},
		{
			StateName:                enum.Finished,
			PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
			AmountReceived:           model.NewBigInt(big.NewInt(payAmount * factor)),
			TransactionConfirmations: 6,
		},
	}
	for i, paymentState := range account.Payments[0].PaymentStates {
		if paymentState.StateName != expectedStates[i].StateName ||
			paymentState.PayAmount.Cmp(&expectedStates[i].PayAmount.Int) != 0 ||
			paymentState.AmountReceived.Cmp(&expectedStates[i].AmountReceived.Int) != 0 ||
			paymentState.TransactionConfirmations != expectedStates[i].TransactionConfirmations {
			t.Errorf("Expected: %v got %v", expectedStates[i].StateName, paymentState.StateName)
			t.Errorf("Expected: %s got %s", expectedStates[i].PayAmount.String(), paymentState.PayAmount.String())
			t.Errorf("Expected: %s got %s", expectedStates[i].AmountReceived.String(), paymentState.AmountReceived.String())
			t.Errorf("Expected: %d got %d", expectedStates[i].TransactionConfirmations, paymentState.TransactionConfirmations)
		}
		if (paymentState.StateName == enum.Forwarded || paymentState.StateName == enum.Finished) && paymentState.TransactionID == "" {
			t.Errorf("Expected transactionID not to be empty, but got %s", paymentState.TransactionID)
		}
	}

	if account.Payments[0].Confirmations != 6 {
		t.Errorf("Expected confirmations %d, but got %d", 6, account.Payments[0].Confirmations)
	}

	chaingateBalance, err := chaingateClient.GetBalance("*")
	if err != nil {
		return
	}

	decodedAddress, err := btcutil.DecodeAddress(testPayment.UserWallet, &chaincfg.RegressionNetParams)
	merchantBalance, err := buyerClient.ListUnspentMinMaxAddresses(6, 999999, []btcutil.Address{decodedAddress})

	hash, err := chainhash.NewHashFromStr(account.Payments[0].PaymentStates[3].TransactionID)
	if err != nil {
		t.Error(err)
	}
	transaction, err := chaingateClient.GetTransaction(hash)
	if err != nil {
		t.Error(err)
	}

	amount := big.NewFloat(merchantBalance[0].Amount)
	fee := big.NewFloat(transaction.Fee)
	//Fee is a negative number!!
	totalSent := amount.Sub(amount, fee)

	a := big.NewFloat(payAmount)
	b := big.NewFloat(0.99)
	forwardAmount := a.Mul(a, b)
	diff, _ := forwardAmount.Sub(forwardAmount, totalSent).Float64()
	if diff > 0.000000001 {
		t.Errorf("Expected unspent amount %f, but got %f", forwardAmount, merchantBalance[0].Amount)
	}

	if chaingateBalance.ToBTC() != payAmount*0.01 {
		t.Errorf("Expected chainganeBalance to be %f, bug got %f", payAmount*0.01, chaingateBalance.ToBTC())
	}

	if account.Used {
		t.Errorf("Expected account to be free")
	}
}
