package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Ryohskay/portacconti/internal/service"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/webhook"
)

type PaymentHandler struct {
	booking              *service.BookingService
	stripeWebhookSecret  string
}

func NewPaymentHandler(booking *service.BookingService, stripeWebhookSecret string) *PaymentHandler {
	return &PaymentHandler{booking: booking, stripeWebhookSecret: stripeWebhookSecret}
}

// POST /api/v1/payments/webhook  (Stripe)
func (h *PaymentHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	const maxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "error reading request body")
		return
	}

	sig := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(payload, sig, h.stripeWebhookSecret)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid stripe signature")
		return
	}

	switch event.Type {
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			respondError(w, http.StatusBadRequest, "failed to parse payment intent")
			return
		}
		if err := h.booking.ConfirmBooking(r.Context(), pi.ID, event.ID); err != nil {
			// Log but return 200 to prevent Stripe retry storms
			// In production, use structured logging here
			_ = err
		}

	case "payment_intent.payment_failed":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err == nil {
			// The slot will be released by the expiry job or can be released immediately
			// For now, the expiry job handles it within 30 minutes
			_ = pi
		}
	}

	// Always return 200 to acknowledge receipt
	w.WriteHeader(http.StatusOK)
}
