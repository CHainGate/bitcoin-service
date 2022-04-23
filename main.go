package main

import (
	"fmt"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	bchdcaincfg "github.com/gcash/bchd/chaincfg"
	bchdrpcclient "github.com/gcash/bchd/rpcclient"
	"github.com/gcash/bchutil"
	"log"
)

func main() {
	//createAddress()

}

func createBchdClient() *bchdrpcclient.Client {
	connCfg := &bchdrpcclient.ConnConfig{
		Host:         "localhost:8332",
		User:         "user",
		Pass:         "pass",
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := bchdrpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown()

	return client
}

func createBtcdClient() *rpcclient.Client {
	connCfg := &rpcclient.ConnConfig{
		Host:         "localhost:18332",
		User:         "user",
		Pass:         "pass",
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}

	defer client.Shutdown()
	return client
}

func sendAmount(client *rpcclient.Client, straddress string) {
	address, err := btcutil.DecodeAddress("tb1qjfkmn6ytt3xdy4cx562qwa2cuzx5r228zxve9r", &chaincfg.TestNet3Params)
	t, err := client.SendToAddress(address, btcutil.Amount(1234))
	if err != nil {
		return
	}
	fmt.Println(t.String())
}

func sendAmountMinusFees(client *bchdrpcclient.Client, straddress string) {
	address, err := bchutil.DecodeAddress("tb1qjfkmn6ytt3xdy4cx562qwa2cuzx5r228zxve9r", &bchdcaincfg.TestNet3Params)
	tx, err := client.SendToAddressCommentSubFee(address, bchutil.Amount(1234), "", "", true)
	if err != nil {
		return
	}
	fmt.Println(tx.String())
}

func getTransactionByLabel(client *rpcclient.Client) []btcjson.ListTransactionsResult {
	transactions, err := client.ListTransactions("new-account")
	if err != nil {
		log.Fatal(err)
	}

	return transactions
}

func getAddressReceivedAmount(client *rpcclient.Client, straddress string) btcutil.Amount {
	address, err := btcutil.DecodeAddress("tb1qjfkmn6ytt3xdy4cx562qwa2cuzx5r228zxve9r", &chaincfg.TestNet3Params)
	if err != nil {
		return btcutil.Amount(0)
	}
	amount, err := client.GetReceivedByAddress(address)
	return amount
}

func createNewAddress(client *rpcclient.Client, label string) string {
	newAddress, err := client.GetNewAddress(label)
	if err != nil {
		log.Fatal(err)
	}

	return newAddress.String()
}

/*func createAddress() {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		log.Fatal(err)
	}

	privBytes := priv.Serialize()
	fmt.Printf("private key [bytes]:\n%v\n\n", privBytes)
	fmt.Printf("private key [hex]:\n%s\n\n", hex.EncodeToString(privBytes))
	fmt.Printf("private key [base58]:\n%s\n\n", base58.Encode(privBytes))


	wif, err := btcutil.NewWIF(priv, &chaincfg.TestNet3Params, true)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("private key [wif] (compressed):\n%s\n\n", wif.String())


	pub := priv.PubKey()
	cmpPubBytes := pub.SerializeCompressed()
	fmt.Printf("public key bytes (compressed):\n%v\n\n", cmpPubBytes)                       // [3 210 ... 69 118]
	fmt.Printf("public key [hex] (compressed):\n%s\n\n", hex.EncodeToString(cmpPubBytes))   // 03d28f502980c5e874c3dd2e4aff019b18e3bef83b5828cf974ffc87c8b0f94576
	fmt.Printf("public key [base58] (compressed):\n%s\n\n", base58.Encode(cmpPubBytes))

	key, err := btcutil.NewAddressPubKey(cmpPubBytes, &chaincfg.TestNet3Params)
	if err != nil {
		return
	}
	encCmpAddr := key.EncodeAddress()
	fmt.Printf("address [base58] (compressed):\n%s\n\n", encCmpAddr)
}*/
