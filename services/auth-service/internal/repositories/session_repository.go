package repositories

import (
	"auth-service/internal/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type SessionRepository interface {
	CreateSession(session *models.Session) error
	GetSessionByToken(tokenHash string) (*models.Session, error)
	GetSessionByRefreshToken(refreshTokenHash string) (*models.Session, error)
	UpdateSession(session *models.Session) error
	RevokeSession(sessionID uuid.UUID) error
	RevokeAllUserSessions(userID uuid.UUID) error
	CleanupExpiredSessions() error
	
	// Redis-based token management
	StoreRefreshToken(userID uuid.UUID, tokenHash string, expiry time.Duration) error
	GetRefreshTokenData(tokenHash string) (string, error)
	DeleteRefreshToken(tokenHash string) error
	BlacklistToken(tokenHash string, expiry time.Duration) error
	IsTokenBlacklisted(tokenHash string) (bool, error)
}

type sessionRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewSessionRepository(db *gorm.DB, redisClient *redis.Client) SessionRepository {
	return &sessionRepository{
		db:    db,
		redis: redisClient,
	}
}

func (r *sessionRepository) CreateSession(session *models.Session) error {
	return r.db.Create(session).Error
}

func (r *sessionRepository) GetSessionByToken(tokenHash string) (*models.Session, error) {
	var session models.Session
	err := r.db.Where("token_hash = ? AND is_revoked = ? AND expires_at > ?", 
		tokenHash, false, time.Now()).First(&session).Error
	
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("session not found")
		}
		return nil, err
	}
	return &session, nil
}

func (r *sessionRepository) GetSessionByRefreshToken(refreshTokenHash string) (*models.Session, error) {
	var session models.Session
	err := r.db.Where("refresh_token_hash = ? AND is_revoked = ? AND expires_at > ?", 
		refreshTokenHash, false, time.Now()).First(&session).Error
	
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("session not found")
		}
		return nil, err
	}
	return &session, nil
}

func (r *sessionRepository) UpdateSession(session *models.Session) error {
	return r.db.Save(session).Error
}

func (r *sessionRepository) RevokeSession(sessionID uuid.UUID) error {
	return r.db.Model(&models.Session{}).
		Where("id = ?", sessionID).
		Update("is_revoked", true).Error
}

func (r *sessionRepository) RevokeAllUserSessions(userID uuid.UUID) error {
	return r.db.Model(&models.Session{}).
		Where("user_id = ?", userID).
		Update("is_revoked", true).Error
}

func (r *sessionRepository) CleanupExpiredSessions() error {
	return r.db.Where("expires_at < ? OR is_revoked = ?", time.Now(), true).
		Delete(&models.Session{}).Error
}

// Redis-based token management
func (r *sessionRepository) StoreRefreshToken(userID uuid.UUID, tokenHash string, expiry time.Duration) error {
	ctx := context.Background()
	
	tokenData := map[string]interface{}{
		"user_id":    userID.String(),
		"created_at": time.Now(),
	}
	
	data, err := json.Marshal(tokenData)
	if err != nil {
		return err
	}
	
	key := fmt.Sprintf("refresh_token:%s", tokenHash)
	return r.redis.Set(ctx, key, data, expiry).Err()
}

func (r *sessionRepository) GetRefreshTokenData(tokenHash string) (string, error) {
	ctx := context.Background()
	key := fmt.Sprintf("refresh_token:%s", tokenHash)
	
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", errors.New("refresh token not found")
		}
		return "", err
	}
	
	var tokenData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &tokenData); err != nil {
		return "", err
	}
	
	userID, ok := tokenData["user_id"].(string)
	if !ok {
		return "", errors.New("invalid token data")
	}
	
	return userID, nil
}

func (r *sessionRepository) DeleteRefreshToken(tokenHash string) error {
	ctx := context.Background()
	key := fmt.Sprintf("refresh_token:%s", tokenHash)
	return r.redis.Del(ctx, key).Err()
}

func (r *sessionRepository) BlacklistToken(tokenHash string, expiry time.Duration) error {
	ctx := context.Background()
	key := fmt.Sprintf("blacklist:%s", tokenHash)
	return r.redis.Set(ctx, key, "1", expiry).Err()
}

func (r *sessionRepository) IsTokenBlacklisted(tokenHash string) (bool, error) {
	ctx := context.Background()
	key := fmt.Sprintf("blacklist:%s", tokenHash)
	
	_, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	
	return true, nil
}