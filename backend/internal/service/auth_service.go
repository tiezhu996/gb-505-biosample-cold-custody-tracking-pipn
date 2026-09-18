package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"biosample-cold-custody-tracking/backend/internal/constants"
	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/model"
	"biosample-cold-custody-tracking/backend/internal/repository"
	"biosample-cold-custody-tracking/backend/internal/util"
)

type AuthService interface {
	Login(context.Context, dto.LoginRequest) (*dto.LoginResponse, error)
	CurrentUser(context.Context, uint) (*dto.UserView, error)
	Seed(context.Context) error
}

type authService struct {
	repo   repository.UserRepository
	secret string
	ttl    time.Duration
}

func NewAuthService(repo repository.UserRepository, secret string, ttl time.Duration) AuthService {
	return &authService{repo: repo, secret: secret, ttl: ttl}
}

func (s *authService) Login(ctx context.Context, input dto.LoginRequest) (*dto.LoginResponse, error) {
	user, err := s.repo.FindByUsername(ctx, input.Username)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !util.ComparePassword(user.PasswordHash, input.Password)) {
		return nil, &util.APIError{Status: 401, Code: "INVALID_CREDENTIALS", Message: "用户名或密码错误"}
	}
	if err != nil {
		return nil, err
	}
	if !user.Active {
		return nil, &util.APIError{Status: 403, Code: "USER_DISABLED", Message: "用户已停用"}
	}
	token, expires, err := util.SignToken(s.secret, s.ttl, user.ID, user.Username, user.DisplayName, user.Role)
	if err != nil {
		return nil, fmt.Errorf("sign token: %w", err)
	}
	return &dto.LoginResponse{Token: token, ExpiresAt: expires.Unix(), User: userView(user)}, nil
}

func (s *authService) CurrentUser(ctx context.Context, id uint) (*dto.UserView, error) {
	user, err := s.repo.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	view := userView(user)
	return &view, nil
}

func (s *authService) Seed(ctx context.Context) error {
	count, err := s.repo.Count(ctx)
	if err != nil || count > 0 {
		return err
	}
	seeds := []struct {
		username, name, password string
		role                     constants.Role
	}{
		{"admin", "样本库管理员", "admin123", constants.RoleAdmin},
		{"receiver", "样本接收员", "receive123", constants.RoleReceiver},
		{"custodian", "冻存保管员", "custody123", constants.RoleCustodian},
		{"reviewer", "协议复核员", "review123", constants.RoleReviewer},
		{"auditor", "链路审计员", "audit123", constants.RoleAuditor},
	}
	for _, seed := range seeds {
		hash, hashErr := util.HashPassword(seed.password)
		if hashErr != nil {
			return hashErr
		}
		if createErr := s.repo.Create(ctx, &model.User{Username: seed.username, DisplayName: seed.name, PasswordHash: hash, Role: seed.role, Active: true}); createErr != nil {
			return createErr
		}
	}
	return nil
}

func userView(user *model.User) dto.UserView {
	return dto.UserView{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Permissions: user.Role.Permissions(),
	}
}
