package controller

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wujunqi/rc_wujunqi/internal/config"
)

func TestAuthMiddlewareAcceptsValidSignature(t *testing.T) {
	router := authTestRouter()
	body := `{"vendor":"crm"}`
	req := signedAuthRequest(t, http.MethodPost, "/notifications", body, "nonce-1", "payment-secret-for-local-dev")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAuthMiddlewareRejectsInvalidSignature(t *testing.T) {
	router := authTestRouter()
	req := signedAuthRequest(t, http.MethodPost, "/notifications", `{"vendor":"crm"}`, "nonce-2", "wrong-secret")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestAuthMiddlewareRejectsExpiredTimestamp(t *testing.T) {
	router := authTestRouter()
	req := signedAuthRequestAt(t, http.MethodPost, "/notifications", `{"vendor":"crm"}`, "nonce-expired", "payment-secret-for-local-dev", time.Now().Add(-10*time.Minute))
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestAuthMiddlewareRejectsReplay(t *testing.T) {
	router := authTestRouter()
	body := `{"vendor":"crm"}`
	first := signedAuthRequest(t, http.MethodPost, "/notifications", body, "nonce-3", "payment-secret-for-local-dev")
	firstResp := httptest.NewRecorder()
	router.ServeHTTP(firstResp, first)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("expected first request 200, got %d", firstResp.Code)
	}

	second := signedAuthRequest(t, http.MethodPost, "/notifications", body, "nonce-3", "payment-secret-for-local-dev")
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, second)
	if secondResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected replay 401, got %d", secondResp.Code)
	}
}

func authTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(NewAuthMiddleware(config.DefaultAppCredentials(), NewMemoryNonceStore(), 5*time.Minute))
	router.POST("/notifications", func(ctx *gin.Context) {
		if AuthAppID(ctx) != "biz-payment" {
			ctx.Status(http.StatusInternalServerError)
			return
		}
		ctx.Status(http.StatusOK)
	})
	return router
}

func signedAuthRequest(t *testing.T, method, path, body, nonce, secret string) *http.Request {
	t.Helper()
	return signedAuthRequestAt(t, method, path, body, nonce, secret, time.Now())
}

func signedAuthRequestAt(t *testing.T, method, path, body, nonce, secret string, requestTime time.Time) *http.Request {
	t.Helper()
	timestamp := strconvFormatUnix(requestTime.Unix())
	signature := hex.EncodeToString(requestSignature(method, path, "biz-payment", timestamp, nonce, []byte(body), secret))
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Id", "biz-payment")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signature)
	return req
}

func strconvFormatUnix(value int64) string {
	return strconv.FormatInt(value, 10)
}
