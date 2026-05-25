package auth

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type Claims struct {
	jwt.RegisteredClaims
}

type JWTValidator struct {
	publicKey       *ecdsa.PublicKey
	redisClient     *redis.Client
	blocklistPrefix string
}

func NewJWTValidator(publicKeyPath string, rdb *redis.Client, blocklistPrefix string) (*JWTValidator, error) {
	data, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	pubKey, err := jwt.ParseECPublicKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ECDSA public key: %w", err)
	}

	return &JWTValidator{
		publicKey:       pubKey,
		redisClient:     rdb,
		blocklistPrefix: blocklistPrefix,
	}, nil
}

// NewJWTValidatorFromKey creates a validator directly from an ecdsa.PublicKey. Useful for testing.
func NewJWTValidatorFromKey(pubKey *ecdsa.PublicKey, rdb *redis.Client, blocklistPrefix string) *JWTValidator {
	return &JWTValidator{
		publicKey:       pubKey,
		redisClient:     rdb,
		blocklistPrefix: blocklistPrefix,
	}
}

func (v *JWTValidator) ValidateToken(ctx context.Context, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Enforce ES256 algorithm
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok || token.Method.Alg() != jwt.SigningMethodES256.Name {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("token parsing/validation failed: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Check Redis blocklist if Redis client is available
	if v.redisClient != nil && claims.ID != "" {
		key := v.blocklistPrefix + claims.ID
		exists, err := v.redisClient.Exists(ctx, key).Result()
		if err != nil {
			// If Redis is temporarily down/errors, we should log or decide whether to fail closed or open.
			// The spec says "Stateful — blocklist stored in Redis". Let's fail closed (reject) or log and proceed.
			// Typically, blocklist checks should be strict, so we fail closed or reject. Let's return error.
			return nil, fmt.Errorf("failed to check blocklist: %w", err)
		}
		if exists > 0 {
			return nil, fmt.Errorf("token is blocklisted")
		}
	}

	return claims, nil
}
