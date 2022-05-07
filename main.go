package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/CHainGate/bitcoin-service/internal/repository"

	"github.com/CHainGate/bitcoin-service/internal/service"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/CHainGate/bitcoin-service/openApi"
)

func main() {
	utils.NewOpts()
	accountRepo, paymentRepo, err := repository.SetupDatabase()
	if err != nil {
		fmt.Println(err)
	}

	//TODO: initial reconciliation with blockchain data, maybe we missed some transactions

	bitcoinService, err := service.NewBitcoinService(accountRepo, paymentRepo)
	if err != nil {
		fmt.Println(err)
	}

	NotificationApiService := service.NewNotificationApiService()
	NotificationApiController := openApi.NewNotificationApiController(NotificationApiService)

	PaymentApiService := service.NewPaymentApiService(bitcoinService)
	PaymentApiController := openApi.NewPaymentApiController(PaymentApiService)

	router := openApi.NewRouter(NotificationApiController, PaymentApiController)

	// https://ribice.medium.com/serve-swaggerui-within-your-golang-application-5486748a5ed4
	sh := http.StripPrefix("/api/swaggerui/", http.FileServer(http.Dir("./swaggerui/")))
	router.PathPrefix("/api/swaggerui/").Handler(sh)

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(utils.Opts.ServerPort), router))
}
