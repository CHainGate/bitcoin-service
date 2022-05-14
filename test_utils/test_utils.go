package testutils

import (
	"encoding/json"
	"fmt"
	"github.com/CHainGate/bitcoin-service/internal/repository"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"log"
	"math/big"
	"strings"
	"time"
)

func DbTestSetup(pool *dockertest.Pool) (*dockertest.Resource, repository.IAccountRepository, repository.IPaymentRepository, error) {
	ressource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       "test-db",
		Repository: "postgres",
		Tag:        "14-alpine",
		Env: []string{
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_USER=user",
			"POSTGRES_DB=testdb",
		},
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	utils.NewOpts()
	utils.Opts.DbHost = "localhost"
	utils.Opts.DbUser = "user"
	utils.Opts.DbPassword = "secret"
	utils.Opts.DbName = "testdb"
	utils.Opts.DbPort = ressource.GetPort("5432/tcp")

	if err != nil {
		return nil, nil, nil, err
	}

	ressource.Expire(120) // Tell docker to hard kill the container in 120 seconds

	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	pool.MaxWait = 120 * time.Second
	var accountRepo repository.IAccountRepository
	var paymentRepo repository.IPaymentRepository
	if err = pool.Retry(func() error {
		accountRepo, paymentRepo, err = repository.SetupDatabase()
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, nil, nil, err
	}
	return ressource, accountRepo, paymentRepo, nil
}

type BitcoinNodeTestSetupResult struct {
	ChaingateClient    *rpcclient.Client
	BuyerClient        *rpcclient.Client
	ChaingateRessource *dockertest.Resource
	BuyerRessource     *dockertest.Resource
}

func BitcoinNodeTestSetup(pool *dockertest.Pool) (*BitcoinNodeTestSetupResult, error) {
	// pulls an image, creates a container based on it and runs it
	fmt.Println("build and run miner-image")
	chaingate, err := pool.BuildAndRun("chaingate-image", "..\\..\\test_utils\\docker\\Dockerfile", []string{})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
		return nil, err
	}

	fmt.Println("build and run buyer-image")
	buyer, err := pool.BuildAndRun("buyer-image", "..\\..\\test_utils\\docker\\Dockerfile", []string{})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
		return nil, err
	}

	chaingateHostRPC := chaingate.GetHostPort("18444/tcp")
	chaingateHost := chaingate.GetPort("18443/tcp")
	buyerHostRPC := buyer.GetHostPort("18444/tcp")
	buyerHost := buyer.GetPort("18443/tcp")
	fmt.Println(chaingateHostRPC)
	fmt.Println(chaingateHost)
	fmt.Println(buyerHostRPC)
	fmt.Println(buyerHost)

	connCfg := &rpcclient.ConnConfig{
		Host:         chaingateHostRPC,
		User:         "user",
		Pass:         "pass",
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	chaingateClient, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatalf("chaingateClient: %s", err)
		return nil, err
	}

	connCfg2 := &rpcclient.ConnConfig{
		Host:         buyerHostRPC,
		User:         "user",
		Pass:         "pass",
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	buyerClient, err := rpcclient.New(connCfg2, nil)
	if err != nil {
		log.Fatalf("buyerClient: %s", err)
		return nil, err
	}

	time.Sleep(2 * time.Second)

	err = chaingateClient.AddNode("host.docker.internal:"+buyerHost, rpcclient.ANAdd)
	if err != nil {
		log.Fatalf("chaingateClient addNode: %s", err)
		return nil, err
	}

	time.Sleep(2 * time.Second)

	err = buyerClient.AddNode("host.docker.internal:"+chaingateHost, rpcclient.ANAdd)
	if err != nil {
		log.Fatalf("buyerClient addNode, %s", err)
	}

	info, err := buyerClient.GetPeerInfo()
	if err != nil {
		return nil, err
	}
	fmt.Println("info:", info)
	info2, err := chaingateClient.GetPeerInfo()
	if err != nil {
		return nil, err
	}
	fmt.Println("info:", info2)

	_, err = buyerClient.CreateWallet("my-wallet")
	if err != nil {
		log.Fatalf("createwallet: %s", err)
	}
	address, err := buyerClient.GetNewAddress("")
	if err != nil {
		log.Fatalf("getnewaddress: %s", err)
	}
	fmt.Println("address:", address)

	var maxTries int64
	maxTries = 1000000 //default
	toAddress, err := buyerClient.GenerateToAddress(101, address, &maxTries)
	if err != nil {
		log.Fatalf("generatetoaddress: %s", err)
	}
	fmt.Println("toAddress:", toAddress)
	fmt.Println("toAddress:", len(toAddress))
	balances, err := buyerClient.GetBalance("*")
	if err != nil {
		log.Fatalf("getwalletinfo%s", err)
	}
	fmt.Println("balance:", balances.ToBTC())

	_, err = chaingateClient.CreateWallet("chaingate-wallet")
	if err != nil {
		log.Fatalf("chaingate createwallet: %s", err)
	}

	result := &BitcoinNodeTestSetupResult{
		ChaingateClient:    chaingateClient,
		BuyerClient:        buyerClient,
		ChaingateRessource: chaingate,
		BuyerRessource:     buyer,
	}
	return result, nil

	/*buyerAddress, err := buyerClient.GetNewAddress("")
	if err != nil {
		log.Fatalf("buyer buyerAddress: %s", err)
	}

	fmt.Println("buyerAddress:", buyerAddress)

	balance, err := buyerClient.GetBalance("*")
	if err != nil {
		log.Fatalf("buyer balance: %s", err)
	}
	fmt.Println("buyer balance: ", balance.ToBTC())

	amount, err := btcutil.NewAmount(1)
	if err != nil {
		log.Fatalf("buyer balance: %s", err)
	}
	sendToAddress, err := chaingateClient.SendToAddress(buyerAddress, amount)
	if err != nil {
		log.Fatalf("buyer balance: %s", err)
	}

	fmt.Println("sendToAddres:", sendToAddress.String())
	transaction, err := chaingateClient.GetTransaction(sendToAddress)
	if err != nil {
		log.Fatalf("miner GetTransaction: %s", err)
	}
	fmt.Println("sendToAddres:", transaction.Amount, transaction.Confirmations, transaction.Details[0])

	toAddress2, err := chaingateClient.GenerateToAddress(10, address, &maxTries)
	if err != nil {
		log.Fatalf("generatetoaddress: %s", err)
	}
	fmt.Println("toAddress:", toAddress2)
	fmt.Println("toAddress:", len(toAddress2))

	balance2, err := buyerClient.GetReceivedByAddressMinConf(buyerAddress, 0)
	if err != nil {
		log.Fatalf("buyer balance: %s", err)
	}
	fmt.Println("buyer balance: ", balance2.ToBTC())*/
	//defer client.Shutdown()
}

func SendToAddress(client *rpcclient.Client, address string, amount *big.Int) (string, error) {
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

// pulls an image, creates a container based on it and runs it
/*fmt.Println("build and run miner-image")
chaingate, err := pool.BuildAndRun("chaingate-image", ".\\docker\\Dockerfile",[]string{})
if err != nil {
	log.Fatalf("Could not start resource: %s", err)
}

fmt.Println("build and run buyer-image")
buyer, err := pool.BuildAndRun("buyer-image", ".\\docker\\Dockerfile",[]string{})
if err != nil {
	log.Fatalf("Could not start resource: %s", err)
}

chaingateHostRPC := chaingate.GetHostPort("18444/tcp")
chaingateHost := chaingate.GetPort("18443/tcp")
buyerHostRPC := buyer.GetHostPort("18444/tcp")
buyerHost := buyer.GetPort("18443/tcp")
fmt.Println(chaingateHostRPC)
fmt.Println(chaingateHost)
fmt.Println(buyerHostRPC)
fmt.Println(buyerHost)

connCfg := &rpcclient.ConnConfig{
	Host:         chaingateHostRPC,
	User:         "user",
	Pass:         "pass",
	HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
	DisableTLS:   true, // Bitcoin core does not provide TLS by default
}
chaingateClient, err := rpcclient.New(connCfg, nil)
if err != nil {
	log.Fatalf("chaingateClient: %s", err)
}

connCfg2 := &rpcclient.ConnConfig{
	Host:         buyerHostRPC,
	User:         "user",
	Pass:         "pass",
	HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
	DisableTLS:   true, // Bitcoin core does not provide TLS by default
}
buyerClient, err := rpcclient.New(connCfg2, nil)
if err != nil {
	log.Fatalf("buyerClient: %s", err)
}

time.Sleep(2 * time.Second)

err = chaingateClient.AddNode("host.docker.internal:"+ buyerHost,  rpcclient.ANAdd)
if err != nil {
	log.Fatalf("chaingateClient addNode: %s", err)
}

time.Sleep(2 * time.Second)

err = buyerClient.AddNode("host.docker.internal:"+chaingateHost,  rpcclient.ANAdd)
if err != nil {
	log.Fatalf("buyerClient addNode, %s", err)
}

info, err := buyerClient.GetPeerInfo()
if err != nil {
	return
}
fmt.Println("info:", info)
info2, err := chaingateClient.GetPeerInfo()
if err != nil {
	return
}
fmt.Println("info:", info2)

_, err = buyerClient.CreateWallet("my-wallet")
if err != nil {
	log.Fatalf("createwallet: %s", err)
}
address, err := buyerClient.GetNewAddress("")
if err != nil {
	log.Fatalf("getnewaddress: %s", err)
}
fmt.Println("address:", address)

var maxTries int64
maxTries = 1000000 //default
toAddress, err := buyerClient.GenerateToAddress(101, address, &maxTries)
if err != nil {
	log.Fatalf("generatetoaddress: %s", err)
}
fmt.Println("toAddress:", toAddress)
fmt.Println("toAddress:", len(toAddress))
balances, err := buyerClient.GetBalance("*")
if err != nil {
	log.Fatalf("getwalletinfo%s", err)
}
fmt.Println("balance:", balances.ToBTC())

_, err = chaingateClient.CreateWallet("chaingate-wallet")
if err != nil {
	log.Fatalf("chaingate createwallet: %s", err)
}*/

/*buyerAddress, err := buyerClient.GetNewAddress("")
if err != nil {
	log.Fatalf("buyer buyerAddress: %s", err)
}

fmt.Println("buyerAddress:", buyerAddress)

balance, err := buyerClient.GetBalance("*")
if err != nil {
	log.Fatalf("buyer balance: %s", err)
}
fmt.Println("buyer balance: ", balance.ToBTC())

amount, err := btcutil.NewAmount(1)
if err != nil {
	log.Fatalf("buyer balance: %s", err)
}
sendToAddress, err := chaingateClient.SendToAddress(buyerAddress, amount)
if err != nil {
	log.Fatalf("buyer balance: %s", err)
}

fmt.Println("sendToAddres:", sendToAddress.String())
transaction, err := chaingateClient.GetTransaction(sendToAddress)
if err != nil {
	log.Fatalf("miner GetTransaction: %s", err)
}
fmt.Println("sendToAddres:", transaction.Amount, transaction.Confirmations, transaction.Details[0])

toAddress2, err := chaingateClient.GenerateToAddress(10, address, &maxTries)
if err != nil {
	log.Fatalf("generatetoaddress: %s", err)
}
fmt.Println("toAddress:", toAddress2)
fmt.Println("toAddress:", len(toAddress2))

balance2, err := buyerClient.GetReceivedByAddressMinConf(buyerAddress, 0)
if err != nil {
	log.Fatalf("buyer balance: %s", err)
}
fmt.Println("buyer balance: ", balance2.ToBTC())*/
//defer client.Shutdown()
