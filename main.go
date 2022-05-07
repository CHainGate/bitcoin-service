package main

import (
	"fmt"
	"github.com/CHainGate/bitcoin-service/internal/repository"
	"log"
	"net/http"
	"strconv"

	"github.com/CHainGate/bitcoin-service/internal/service"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"github.com/CHainGate/bitcoin-service/openApi"
)

func main() {
	utils.NewOpts()
	err := repository.SetupDatabase()
	if err != nil {
		fmt.Println(err)
	}

	NotificationApiService := service.NewNotificationApiService()
	NotificationApiController := openApi.NewNotificationApiController(NotificationApiService)

	PaymentApiService := service.NewPaymentApiService()
	PaymentApiController := openApi.NewPaymentApiController(PaymentApiService)

	router := openApi.NewRouter(NotificationApiController, PaymentApiController)

	// https://ribice.medium.com/serve-swaggerui-within-your-golang-application-5486748a5ed4
	sh := http.StripPrefix("/api/swaggerui/", http.FileServer(http.Dir("./swaggerui/")))
	router.PathPrefix("/api/swaggerui/").Handler(sh)

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(utils.Opts.ServerPort), router))
}
