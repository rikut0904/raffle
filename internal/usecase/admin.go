package usecase

import (
	"context"
	"presentation-raffle/internal/domain/entity"
	"presentation-raffle/internal/domain/repository"
	"presentation-raffle/internal/domain/service"
	"time"
)

type AdminUsecase struct {
	userRepo   repository.UserRepository
	raffleRepo repository.RaffleRepository
	verifier   service.TokenVerifier
}

func NewAdminUsecase(userRepo repository.UserRepository, raffleRepo repository.RaffleRepository, verifier service.TokenVerifier) *AdminUsecase {
	return &AdminUsecase{
		userRepo:   userRepo,
		raffleRepo: raffleRepo,
		verifier:   verifier,
	}
}

func (u *AdminUsecase) VerifyToken(ctx context.Context, idToken string) (*entity.User, error) {
	authUser, err := u.verifier.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, err
	}

	user, err := u.userRepo.GetByUID(ctx, authUser.UID)
	if err != nil {
		// New user or error
		user = entity.User{
			UID:           authUser.UID,
			Email:         authUser.Email,
			DisplayName:   authUser.DisplayName,
			PhotoURL:      authUser.PhotoURL,
			Provider:      authUser.Provider,
			EmailVerified: authUser.EmailVerified,
		}
	}

	// Always update / Create
	saved, err := u.userRepo.Upsert(ctx, user)
	if err != nil {
		return nil, err
	}

	return &saved, nil
}

func (u *AdminUsecase) SyncUser(ctx context.Context, authUser entity.AuthenticatedUser) (*entity.User, error) {
	user := entity.User{UID: authUser.UID, Email: authUser.Email, DisplayName: authUser.DisplayName, PhotoURL: authUser.PhotoURL, Provider: authUser.Provider, EmailVerified: authUser.EmailVerified, LastLoginAt: time.Now()}
	saved, err := u.userRepo.Upsert(ctx, user)
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func (u *AdminUsecase) GetUser(ctx context.Context, uid string) (*entity.User, error) {
	user, err := u.userRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *AdminUsecase) ListRaffles(ctx context.Context, userUID string) ([]entity.Raffle, error) {
	return u.raffleRepo.ListByUser(ctx, userUID)
}

func (u *AdminUsecase) SaveRaffle(ctx context.Context, raffle entity.Raffle) (entity.Raffle, error) {
	return u.raffleRepo.Save(ctx, raffle)
}

func (u *AdminUsecase) DeleteRaffle(ctx context.Context, id string, userUID string) error {
	return u.raffleRepo.Delete(ctx, id, userUID)
}

func (u *AdminUsecase) GetRaffle(ctx context.Context, userUID string, id string) (entity.Raffle, error) {
	return u.raffleRepo.GetByID(ctx, id, userUID)
}
