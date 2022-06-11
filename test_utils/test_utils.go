package testutils

import (
	"github.com/CHainGate/bitcoin-service/internal/repository"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"log"
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
	utils.Opts.TestWalletPassphrase = "secret"

	if err != nil {
		return nil, nil, nil, err
	}

	err = ressource.Expire(120) // Tell docker to hard kill the container in 120 seconds
	if err != nil {
		return nil, nil, nil, err
	}

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
	chaingate, err := pool.BuildAndRun("chaingate-image", "..\\..\\test_utils\\docker\\Dockerfile", []string{})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
		return nil, err
	}

	buyer, err := pool.BuildAndRun("buyer-image", "..\\..\\test_utils\\docker\\Dockerfile", []string{})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
		return nil, err
	}

	err = chaingate.Expire(120) // Tell docker to hard kill the container in 120 seconds
	if err != nil {
		return nil, err
	}
	err = buyer.Expire(120) // Tell docker to hard kill the container in 120 seconds
	if err != nil {
		return nil, err
	}

	chaingateHostRPC := chaingate.GetHostPort("18444/tcp")
	chaingateHost := chaingate.GetPort("18443/tcp")
	buyerHostRPC := buyer.GetHostPort("18444/tcp")
	buyerHost := buyer.GetPort("18443/tcp")

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

	// wait for nodes to be ready
	time.Sleep(2 * time.Second)

	err = chaingateClient.AddNode("host.docker.internal:"+buyerHost, rpcclient.ANAdd)
	if err != nil {
		log.Fatalf("chaingateClient addNode: %s", err)
		return nil, err
	}

	err = buyerClient.AddNode("host.docker.internal:"+chaingateHost, rpcclient.ANAdd)
	if err != nil {
		log.Fatalf("buyerClient addNode, %s", err)
	}

	_, err = buyerClient.CreateWallet("my-wallet")
	if err != nil {
		log.Fatalf("createwallet: %s", err)
	}

	address, err := buyerClient.GetNewAddress("")
	if err != nil {
		log.Fatalf("getnewaddress: %s", err)
	}

	var maxTries int64
	maxTries = 1000000 //default
	_, err = buyerClient.GenerateToAddress(101, address, &maxTries)
	if err != nil {
		log.Fatalf("generatetoaddress: %s", err)
	}

	_, err = chaingateClient.CreateWallet("chaingate-wallet", rpcclient.WithCreateWalletPassphrase("secret"))
	if err != nil {
		log.Fatalf("chaingate createwallet: %s", err)
	}
	changeAddress, err := chaingateClient.GetRawChangeAddress("bech32")
	if err != nil {
		log.Fatalf("changeAddress: %s", err)
	}

	utils.Opts.TestChangeAddress = changeAddress.EncodeAddress()

	result := &BitcoinNodeTestSetupResult{
		ChaingateClient:    chaingateClient,
		BuyerClient:        buyerClient,
		ChaingateRessource: chaingate,
		BuyerRessource:     buyer,
	}
	return result, nil
}
