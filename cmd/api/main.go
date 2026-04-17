package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ryohskay/portacconti/internal/config"
	"github.com/Ryohskay/portacconti/internal/db"
	"github.com/Ryohskay/portacconti/internal/handler"
	"github.com/Ryohskay/portacconti/internal/repository/postgres"
	"github.com/Ryohskay/portacconti/internal/service"
	stripe "github.com/stripe/stripe-go/v79"
)

func main() {
	// Load .env in development
	loadDotEnv()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	// Stripe
	stripe.Key = cfg.StripeSecretKey

	// Repositories
	userRepo := postgres.NewUserRepo(pool, cfg.EncryptionKey)
	shiftRepo := postgres.NewShiftRepo(pool, cfg.EncryptionKey)
	apptRepo := postgres.NewAppointmentRepo(pool, cfg.EncryptionKey)
	paymentRepo := postgres.NewPaymentRepo(pool)
	msgRepo := postgres.NewMessageRepo(pool, cfg.EncryptionKey)
	questionnaireRepo := postgres.NewQuestionnaireRepo(pool, cfg.EncryptionKey)
	auditRepo := postgres.NewAuditRepo(pool)
	_ = auditRepo // used in middleware where needed

	// Services
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret)

	notifySvc := service.NewNotificationService(
		service.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			User:     cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     cfg.EmailFrom,
		},
		cfg.BaseURL,
		questionnaireRepo,
		userRepo,
	)

	const appointmentPriceJPY = 5000
	bookingSvc := service.NewBookingService(apptRepo, shiftRepo, paymentRepo, userRepo, notifySvc, appointmentPriceJPY)

	// Background expiry job
	go bookingSvc.StartPendingPaymentExpiryJob(ctx)

	// Handlers
	handlers := &handler.Handlers{
		Auth:          handler.NewAuthHandler(authSvc),
		Shifts:        handler.NewShiftHandler(shiftRepo),
		Appointments:  handler.NewAppointmentHandler(apptRepo, userRepo, bookingSvc),
		Payments:      handler.NewPaymentHandler(bookingSvc, cfg.StripeWebhookSecret),
		Patients:      handler.NewPatientHandler(userRepo),
		Messages:      handler.NewMessageHandler(msgRepo, apptRepo, userRepo, notifySvc),
		Questionnaire: handler.NewQuestionnaireHandler(questionnaireRepo),
		Admin:         handler.NewAdminHandler(authSvc, userRepo),
	}

	router := handler.NewRouter(handlers, authSvc)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown: %v", err)
	}
	log.Println("server stopped")
}

func loadDotEnv() {
	// Simple .env loader — does not override existing env vars
	data, err := os.ReadFile(".env")
	if err != nil {
		return // .env is optional
	}
	for _, line := range splitLines(string(data)) {
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		for i, c := range line {
			if c == '=' {
				key := line[:i]
				val := line[i+1:]
				if os.Getenv(key) == "" {
					_ = os.Setenv(key, val)
				}
				break
			}
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
