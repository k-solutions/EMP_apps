package middleware

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/emarchant/rssservice/internal/auth"
	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddleware(t *testing.T) {
	// Generate keys for mock validator
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	validator := auth.NewJWTValidatorFromKey(&privKey.PublicKey, nil, "test:")

	authMw := Auth(validator)

	// A dummy handler that checks for context claims
	handler := authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaims(r.Context())
		if claims == nil {
			t.Error("expected claims in context, got nil")
		} else if claims.Subject != "test-user" {
			t.Errorf("expected subject = test-user, got %s", claims.Subject)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// 1. Missing Authorization Header
	req1 := httptest.NewRequest("GET", "/test", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w1.Code)
	}

	// 2. Invalid Header Format
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Authorization", "InvalidFormat token")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w2.Code)
	}

	// 3. Valid Token
	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "test-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenStr, _ := token.SignedString(privKey)

	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.Header.Set("Authorization", "Bearer "+tokenStr)
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w3.Code)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	loggingMw := Logging(logger)

	handler := loggingMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetLogJobID(r.Context(), "job-log-123")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	}))

	req := httptest.NewRequest("POST", "/parse", nil)
	req.RemoteAddr = "1.2.3.4"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}

	// Parse logged JSON output
	var logOutput map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logOutput)
	if err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	if logOutput["msg"] != "request" {
		t.Errorf("expected msg = request, got %v", logOutput["msg"])
	}
	if logOutput["method"] != "POST" {
		t.Errorf("expected method = POST, got %v", logOutput["method"])
	}
	if logOutput["path"] != "/parse" {
		t.Errorf("expected path = /parse, got %v", logOutput["path"])
	}
	if logOutput["job_id"] != "job-log-123" {
		t.Errorf("expected job_id = job-log-123, got %v", logOutput["job_id"])
	}
	if logOutput["status"] != float64(http.StatusAccepted) {
		t.Errorf("expected status = 202, got %v", logOutput["status"])
	}
	if logOutput["remote_addr"] != "1.2.3.4" {
		t.Errorf("expected remote_addr = 1.2.3.4, got %v", logOutput["remote_addr"])
	}
}
