package service

import (
	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"github.com/CHainGate/bitcoin-service/internal/repository"
	"github.com/CHainGate/bitcoin-service/openApi"
	"github.com/CHainGate/bitcoin-service/test_utils"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/ory/dockertest/v3"
	"gopkg.in/h2non/gock.v1"
	"log"
	"math/big"
	"os"
	"testing"
)

var (
	accountRepo     repository.IAccountRepository
	paymentRepo     repository.IPaymentRepository
	chaingateClient *rpcclient.Client
	buyerClient     *rpcclient.Client
)

const factor = 100000000
const payAmount = 0.003403

var testPaymentState = model.PaymentState{
	StateName:                enum.Waiting,
	PayAmount:                model.NewBigInt(big.NewInt(payAmount * factor)),
	AmountReceived:           model.NewBigInt(big.NewInt(0)),
	TransactionConfirmations: 0,
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

	service := NewBitcoinService(accountRepo, paymentRepo, chaingateClient, nil)

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
}
