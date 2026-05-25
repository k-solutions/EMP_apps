package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

func TestJWTValidator(t *testing.T) {
	// Generate keys dynamically
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	// Create validator
	validator := NewJWTValidatorFromKey(pubKey, nil, "jwt:blocklist:")

	// 1. Valid Token
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "test-jti-1",
			Subject:   "test-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenStr, err := token.SignedString(privKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	ctx := context.Background()
	validatedClaims, err := validator.ValidateToken(ctx, tokenStr)
	if err != nil {
		t.Errorf("failed to validate valid token: %v", err)
	}
	if validatedClaims.Subject != "test-user" {
		t.Errorf("expected subject = test-user, got %s", validatedClaims.Subject)
	}

	// 2. Expired Token
	expiredClaims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "test-jti-2",
			Subject:   "test-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodES256, expiredClaims)
	expiredTokenStr, err := expiredToken.SignedString(privKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	_, err = validator.ValidateToken(ctx, expiredTokenStr)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}

	// 3. Wrong Algorithm (e.g. HS256)
	hsToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	hsTokenStr, err := hsToken.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("failed to sign HS256 token: %v", err)
	}
	_, err = validator.ValidateToken(ctx, hsTokenStr)
	if err == nil {
		t.Error("expected error for wrong signing algorithm, got nil")
	}
}

func TestJWTValidatorBlocklist(t *testing.T) {
	// Try to connect to local Redis for testing the blocklist check
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("skipping Redis blocklist test: local Redis not running")
	}
	defer rdb.Close()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	blocklistPrefix := "test:jwt:blocklist:"
	validator := NewJWTValidatorFromKey(pubKey, rdb, blocklistPrefix)

	jti := "blocked-jti-123"
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   "test-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenStr, err := token.SignedString(privKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// First verify it's valid before blocklisting
	_, err = validator.ValidateToken(ctx, tokenStr)
	if err != nil {
		t.Fatalf("token should be valid before blocklisting: %v", err)
	}

	// Blocklist the token in Redis
	key := blocklistPrefix + jti
	err = rdb.Set(ctx, key, "blocked", 10*time.Second).Err()
	if err != nil {
		t.Fatalf("failed to write to Redis: %v", err)
	}
	defer rdb.Del(ctx, key)

	// Now verify it fails blocklist check
	_, err = validator.ValidateToken(ctx, tokenStr)
	if err == nil {
		t.Error("expected token to be rejected because of blocklist, but succeeded")
	}
}
