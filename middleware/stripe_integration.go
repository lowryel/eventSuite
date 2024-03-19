package middleware

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/paymentintent"
)

// var (
// 	Stripe_Key = os.Getenv("STRIPE_SECRET_KEY")
// )
func HandleCreatePaymentIntent(receipt_total int64, ctx *fiber.Ctx) {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	// Create a PaymentIntent with amount and currency
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(receipt_total),
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		// In the latest version of the API, specifying the `automatic_payment_methods` parameter is optional because Stripe enables its functionality by default.
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}

	pi, err := paymentintent.New(params)
	log.Printf("pi.New: %v", pi.ClientSecret)

	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		log.Printf("pi.New: %v", err)
		return
	}

	// Example: Confirm the payment intent after client-side payment
	paymentConfirmParams := &stripe.PaymentIntentConfirmParams{

		PaymentMethod: stripe.String("pm_card_visa"),
		ReturnURL: stripe.String("https://www.example.com"),
	}
	id := strings.Split(pi.ClientSecret, "_secret")[0]

	pii, err := paymentintent.Confirm(id, paymentConfirmParams)
	if err != nil {
		log.Fatalf("Failed to confirm payment intent: %v", err)
	}

	log.Printf("Payment intent confirmed: %v\n", pii.Status)
}

func CreateCustomer(email, name string) *stripe.Customer{
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	params := &stripe.CustomerParams{
		Name: stripe.String(name),
		Email: stripe.String(email),
	}
	result, err := customer.New(params);
	if err != nil {
		log.Fatalf("Failed to create  a Customer: %v", err)
	}else {
		log.Printf("Other error occurred: %v\n", err)
	}
	return result
}
