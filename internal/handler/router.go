package handler

import (
	"github.com/Ryohskay/portacconti/internal/middleware"
	"github.com/Ryohskay/portacconti/internal/service"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"net/http"
)

type Handlers struct {
	Auth          *AuthHandler
	Shifts        *ShiftHandler
	Appointments  *AppointmentHandler
	Payments      *PaymentHandler
	Patients      *PatientHandler
	Messages      *MessageHandler
	Questionnaire *QuestionnaireHandler
	Admin         *AdminHandler
}

func NewRouter(h *Handlers, authSvc *service.AuthService) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Logger)
	r.Use(middleware.I18n)

	r.Route("/api/v1", func(r chi.Router) {
		// --- Public routes ---
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", h.Auth.Register)
			r.Post("/login", h.Auth.Login)
			r.Post("/refresh", h.Auth.Refresh)
		})

		// Questionnaire (token-auth, no JWT)
		r.Route("/questionnaire", func(r chi.Router) {
			r.Get("/{token}", h.Questionnaire.GetForm)
			r.Post("/{token}", h.Questionnaire.Submit)
		})

		// Available timeslots (patient must browse before login too)
		r.Get("/timeslots/available", h.Shifts.ListAvailableTimeslots)

		// --- Authenticated routes ---
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate(authSvc))

			r.Post("/auth/logout", h.Auth.Logout)
			r.Get("/auth/me", h.Auth.Me)

			// Appointments
			r.Route("/appointments", func(r chi.Router) {
				r.With(middleware.RequireRole("patient")).Post("/", h.Appointments.InitiateBooking)
				r.Get("/", h.Appointments.List)
				r.Get("/{id}", h.Appointments.Get)
				r.With(middleware.RequireRole("manager")).Patch("/{id}", h.Appointments.UpdateByManager)
				r.With(middleware.RequireRole("counsellor", "manager")).Patch("/{id}/status", h.Appointments.UpdateStatus)
				r.With(middleware.RequireRole("manager")).Delete("/{id}", h.Appointments.Cancel)

				// Patient records
				r.With(middleware.RequireRole("counsellor", "manager")).Post("/{id}/records", h.Appointments.AddRecord)
				r.With(middleware.RequireRole("counsellor", "manager")).Get("/{id}/records", h.Appointments.ListRecords)
				r.With(middleware.RequireRole("counsellor", "manager")).Put("/{id}/records/{record_id}", h.Appointments.UpdateRecord)

				// Messages
				r.With(middleware.RequireRole("counsellor", "manager")).Post("/{id}/messages", h.Messages.Send)
				r.Get("/{id}/messages", h.Messages.List)
			})

			// Payments (webhook is public below)
			r.Route("/payments", func(r chi.Router) {
				// Webhook is outside authenticated group
			})

			// Shifts (manager only)
			r.Route("/shifts", func(r chi.Router) {
				r.Use(middleware.RequireRole("manager"))
				r.Post("/", h.Shifts.CreateShift)
				r.Get("/", h.Shifts.ListShifts)
				r.Delete("/{id}", h.Shifts.CloseShift)
				r.Post("/{id}/timeslots", h.Shifts.GenerateTimeslots)
				r.Get("/{id}/timeslots", h.Shifts.ListTimeslotsByShift)
				r.Post("/{id}/counsellors", h.Shifts.AddCounsellor)
				r.Delete("/{id}/counsellors/{counsellor_id}", h.Shifts.RemoveCounsellor)
			})

			// Patients
			r.Route("/patients", func(r chi.Router) {
				r.With(middleware.RequireRole("manager")).Get("/", h.Patients.ListPatients)
				r.Get("/{id}", h.Patients.GetPatient)
				r.Put("/{id}", h.Patients.UpdatePatient)
			})

			// Counsellors list
			r.With(middleware.RequireRole("manager")).Get("/counsellors", h.Patients.ListCounsellors)

			// Admin
			r.Route("/admin", func(r chi.Router) {
				r.Use(middleware.RequireRole("manager"))
				r.Post("/users", h.Admin.CreateStaff)
				r.Get("/users", h.Admin.ListStaff)
				r.Patch("/users/{id}", h.Admin.UpdateStaff)
			})
		})

		// Stripe webhook — outside JWT auth
		r.Post("/payments/webhook", h.Payments.StripeWebhook)
	})

	return r
}
