package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"server-web/model"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAuthUnavailable    = errors.New("auth service unavailable")
	ErrBearerTokenMissing = errors.New("bearer token required")
)

type Service struct {
	db     *gorm.DB
	tokens *TokenManager
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      Identity
}

func NewService(db *gorm.DB, jwtSecret string, tokenTTL time.Duration) (*Service, error) {
	if db == nil {
		return nil, ErrAuthUnavailable
	}
	tokens, err := NewTokenManager(jwtSecret, tokenTTL)
	if err != nil {
		return nil, err
	}
	return &Service{db: db, tokens: tokens}, nil
}

func (s *Service) EnsureInitialAdmin(ctx context.Context, adminPassword string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.User{}).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 || strings.TrimSpace(adminPassword) == "" {
		return false, nil
	}

	hashedPassword, err := HashPassword(adminPassword)
	if err != nil {
		return false, err
	}
	admin := model.User{
		Username: "admin",
		Password: hashedPassword,
		Role:     "admin",
	}
	if err := s.db.WithContext(ctx).Create(&admin).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) Login(ctx context.Context, username string, password string) (LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return LoginResult{}, ErrInvalidCredentials
	}

	var user model.User
	err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}
	if !VerifyPassword(user.Password, password) {
		return LoginResult{}, ErrInvalidCredentials
	}

	identity := Identity{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
	}
	token, expiresAt, err := s.tokens.Generate(identity)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      identity,
	}, nil
}

func (s *Service) AuthenticateBearer(authHeader string) (Identity, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return Identity{}, ErrBearerTokenMissing
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
	if token == "" {
		return Identity{}, ErrBearerTokenMissing
	}
	return s.tokens.Parse(token)
}
