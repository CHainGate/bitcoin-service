package service

import (
	"fmt"
	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"github.com/CHainGate/bitcoin-service/internal/repository"
	"github.com/CHainGate/bitcoin-service/openApi"
	"github.com/CHainGate/bitcoin-service/test_utils"
	"github.com/btcsuite/btcd/chaincfg"
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

var testPaymentState = model.PaymentState{
	StateName:                enum.Waiting,
	PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
	AmountReceived:           model.NewBigInt(big.NewInt(0)),
	TransactionConfirmations: -1,
}

var testPayment = model.Payment{
	Account:               nil,
	UserWallet:            "test-wallet",
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

	service = NewBitcoinService(accountRepo, paymentRepo, chaingateClient, nil)

	//Run tests
	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(bitcoinSetupResult.ChaingateRessource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
	if err := pool.Purge(bitcoinSetupResult.BuyerRessource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
	if err := pool.Purge(dbRessource); err != nil {
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
		Wallet:        "test-wallet",
		Mode:          "test",
	}

	// Act
	payment, err := service.CreateNewPayment(request)
	if err != nil {
		t.Errorf("Create paymend got an error: %s", err)
	}

	// Assert
	if payment.Account.Address == "" ||
		payment.Account.Remainder.Int64() != 0 ||
		payment.UserWallet != testPayment.UserWallet ||
		payment.Mode != testPayment.Mode ||
		payment.PriceAmount != testPayment.PriceAmount ||
		payment.PriceCurrency != testPayment.PriceCurrency ||
		payment.CurrentPaymentStateId.String() == "" ||
		payment.Confirmations != testPayment.Confirmations {
		t.Errorf("Expected payment to be: %v but got %v", testPayment, payment)
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

	fmt.Println(account.Payments[0].CurrentPaymentState.AmountReceived.String())
	fmt.Println(testPaymentState.PayAmount.String())
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
