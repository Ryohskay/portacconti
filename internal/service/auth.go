package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/Ryohskay/portacconti/internal/repository"
	pgrepo "github.com/Ryohskay/portacconti/internal/repository/postgres"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already in use")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrForbidden          = errors.New("insufficient permissions")
)

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Locale string `json:"locale"`
	jwt.RegisteredClaims
}

type AuthService struct {
	users     repository.UserRepository
	jwtSecret []byte
}

func NewAuthService(users repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{users: users, jwtSecret: []byte(jwtSecret)}
}

type RegisterInput struct {
	Email    string
	Password string
	Name     string
	Locale   string
	Role     domain.Role // only patient via self-registration; staff created by manager
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*domain.User, *TokenPair, error) {
	existing, err := s.users.GetByEmail(ctx, in.Email)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		return nil, nil, ErrEmailTaken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	role := in.Role
	if role == "" {
		role = domain.RolePatient
	}
	locale := in.Locale
	if locale == "" {
		locale = "ja"
	}

	u := &domain.User{
		Email:          in.Email,
		HashedPassword: string(hashed),
		Name:           in.Name,
		Role:           role,
		Locale:         locale,
		IsActive:       true,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, nil, fmt.Errorf("create user: %w", err)
	}

	tokens, err := s.issueTokens(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	return u, tokens, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, *TokenPair, error) {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, nil, err
	}
	if u == nil || !u.IsActive {
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.HashedPassword), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := s.issueTokens(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	return u, tokens, nil
}

func (s *AuthService) Refresh(ctx context.Context, rawRefreshToken string) (*domain.User, *TokenPair, error) {
	hash := pgrepo.HashToken(rawRefreshToken)
	userID, valid, err := s.users.GetRefreshToken(ctx, hash)
	if err != nil {
		return nil, nil, err
	}
	if !valid {
		return nil, nil, ErrInvalidToken
	}

	// Rotate: revoke old token
	if err := s.users.RevokeRefreshToken(ctx, hash); err != nil {
		return nil, nil, err
	}

	u, err := s.users.GetByID(ctx, userID)
	if err != nil || u == nil {
		return nil, nil, ErrUserNotFound
	}
	if !u.IsActive {
		return nil, nil, ErrInvalidToken
	}

	tokens, err := s.issueTokens(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	return u, tokens, nil
}

func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	hash := pgrepo.HashToken(rawRefreshToken)
	return s.users.RevokeRefreshToken(ctx, hash)
}

func (s *AuthService) ValidateAccessToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// CreateStaffUser allows managers to create counsellor/manager accounts.
func (s *AuthService) CreateStaffUser(ctx context.Context, in RegisterInput) (*domain.User, error) {
	existing, err := s.users.GetByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	locale := in.Locale
	if locale == "" {
		locale = "ja"
	}

	u := &domain.User{
		Email:          in.Email,
		HashedPassword: string(hashed),
		Name:           in.Name,
		Role:           in.Role,
		Locale:         locale,
		IsActive:       true,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *AuthService) issueTokens(ctx context.Context, u *domain.User) (*TokenPair, error) {
	accessToken, err := s.signAccessToken(u)
	if err != nil {
		return nil, err
	}

	rawRefresh, err := generateSecureToken()
	if err != nil {
		return nil, err
	}

	hash := pgrepo.HashToken(rawRefresh)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := s.users.SaveRefreshToken(ctx, u.ID, hash, expiresAt); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: rawRefresh}, nil
}

func (s *AuthService) signAccessToken(u *domain.User) (string, error) {
	claims := &Claims{
		UserID: u.ID.String(),
		Role:   string(u.Role),
		Locale: u.Locale,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ParseUserID is a helper used by handlers.
func ParseUserID(id string) (uuid.UUID, error) {
	return uuid.Parse(id)
}
