package service

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/Ryohskay/portacconti/internal/repository"
	pgrepo "github.com/Ryohskay/portacconti/internal/repository/postgres"
)

type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

type NotificationService struct {
	smtp          SMTPConfig
	baseURL       string
	questionnaire repository.QuestionnaireRepository
	users         repository.UserRepository
}

func NewNotificationService(
	cfg SMTPConfig,
	baseURL string,
	questionnaire repository.QuestionnaireRepository,
	users repository.UserRepository,
) *NotificationService {
	return &NotificationService{
		smtp:          cfg,
		baseURL:       baseURL,
		questionnaire: questionnaire,
		users:         users,
	}
}

func (n *NotificationService) SendBookingConfirmation(ctx context.Context, appt *domain.Appointment, locale string) {
	patient := appt.Patient
	if patient == nil {
		return
	}

	var subject, body string
	if locale == "ja" {
		subject = "【ご予約確認】カウンセリング予約が完了しました"
		body = fmt.Sprintf(
			"%s 様\n\nカウンセリングのご予約が確定しました。\n\n日時: %s〜%s\n\nご質問がある場合は、ご返信ください。\n\nよろしくお願いいたします。",
			patient.Name,
			appt.Timeslot.StartsAt.Format("2006年01月02日 15:04"),
			appt.Timeslot.EndsAt.Format("15:04"),
		)
	} else {
		subject = "Booking Confirmation – Counselling Appointment"
		body = fmt.Sprintf(
			"Dear %s,\n\nYour counselling appointment has been confirmed.\n\nDate & Time: %s – %s\n\nIf you have any questions, please reply to this email.\n\nBest regards,",
			patient.Name,
			appt.Timeslot.StartsAt.Format("January 2, 2006 15:04"),
			appt.Timeslot.EndsAt.Format("15:04"),
		)
	}

	_ = n.sendEmail(patient.Email, subject, body)

	// Send questionnaire link
	n.SendQuestionnaire(ctx, appt, locale)
}

func (n *NotificationService) SendQuestionnaire(ctx context.Context, appt *domain.Appointment, locale string) {
	if appt.Patient == nil {
		return
	}

	tmpl, err := n.questionnaire.GetActiveTemplate(ctx, locale)
	if err != nil || tmpl == nil {
		return
	}

	rawToken, err := generateSecureToken()
	if err != nil {
		return
	}
	hash := pgrepo.HashToken(rawToken)

	tok := &domain.QuestionnaireToken{
		AppointmentID: appt.ID,
		TemplateID:    tmpl.ID,
		TokenHash:     hash,
		ExpiresAt:     time.Now().Add(72 * time.Hour),
	}
	if err := n.questionnaire.SaveToken(ctx, tok); err != nil {
		return
	}

	link := fmt.Sprintf("%s/questionnaire/%s", n.baseURL, rawToken)

	var subject, body string
	if locale == "ja" {
		subject = "【事前問診票】ご来院前にご記入ください"
		body = fmt.Sprintf(
			"%s 様\n\nカウンセリングの前に、以下のリンクから事前問診票にご記入ください。\n\n%s\n\n※リンクは72時間有効です。",
			appt.Patient.Name, link,
		)
	} else {
		subject = "Pre-visit Questionnaire – Please Complete Before Your Appointment"
		body = fmt.Sprintf(
			"Dear %s,\n\nPlease complete the pre-visit questionnaire before your appointment:\n\n%s\n\nThis link expires in 72 hours.",
			appt.Patient.Name, link,
		)
	}

	_ = n.sendEmail(appt.Patient.Email, subject, body)
}

func (n *NotificationService) AlertNoCounsellor(ctx context.Context, appt *domain.Appointment) {
	managers, err := n.users.ListByRole(ctx, domain.RoleManager)
	if err != nil || len(managers) == 0 {
		return
	}
	for _, m := range managers {
		body := fmt.Sprintf(
			"Warning: Appointment %s has no available counsellor assigned. Please assign one manually.",
			appt.ID,
		)
		_ = n.sendEmail(m.Email, "Action Required: Unassigned Appointment", body)
	}
}

func (n *NotificationService) SendMessageEmail(ctx context.Context, toEmail, senderName, subject, body string) error {
	fullBody := fmt.Sprintf("Message from %s:\n\n%s", senderName, body)
	return n.sendEmail(toEmail, subject, fullBody)
}

func (n *NotificationService) sendEmail(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%s", n.smtp.Host, n.smtp.Port)
	msg := strings.Join([]string{
		"From: " + n.smtp.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	var auth smtp.Auth
	if n.smtp.User != "" && n.smtp.Password != "" {
		auth = smtp.PlainAuth("", n.smtp.User, n.smtp.Password, n.smtp.Host)
	}

	return smtp.SendMail(addr, auth, n.smtp.From, []string{to}, []byte(msg))
}

