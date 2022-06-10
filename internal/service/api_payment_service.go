/*
 * OpenAPI bitcoin service
 *
 * This is the OpenAPI definition of the bitcoin service.
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package service

import (
	"context"
	"net/http"

	"github.com/CHainGate/backend/pkg/enum"

	"github.com/CHainGate/bitcoin-service/openApi"
)

// PaymentApiService is a service that implements the logic for the PaymentApiServicer
// This service should implement the business logic for every endpoint for the PaymentApi API.
// Include any external packages or services that will be required by this service.
type PaymentApiService struct {
	bitcoinService IBitcoinService
}

// NewPaymentApiService creates a default api service
func NewPaymentApiService(bitcoinService IBitcoinService) openApi.PaymentApiServicer {
	return &PaymentApiService{bitcoinService}
}

// CreatePayment - create new payment
func (s *PaymentApiService) CreatePayment(ctx context.Context, paymentRequestDto openApi.PaymentRequestDto) (openApi.ImplResponse, error) {
	payment, err := s.bitcoinService.CreateNewPayment(paymentRequestDto)
	if err != nil {
		return openApi.Response(http.StatusBadRequest, nil), err
	}

	result := openApi.PaymentResponseDto{
		PaymentId:     payment.ID.String(),
		PriceAmount:   payment.PriceAmount,
		PriceCurrency: payment.PriceCurrency.String(),
		PayAddress:    payment.Account.Address,
		PayAmount:     payment.PaymentStates[0].PayAmount.String(),
		PayCurrency:   enum.BTC.String(),
		PaymentState:  payment.PaymentStates[0].StateId.String(),
	}

	return openApi.Response(http.StatusCreated, result), nil
}
