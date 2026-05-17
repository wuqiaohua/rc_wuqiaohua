package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wujunqi/rc_wujunqi/internal/config"
)

const authAppIDKey = "auth_app_id"

func NewAuthMiddleware(credentials map[string]config.AppCredential, nonceStore NonceStore, allowedSkew time.Duration) gin.HandlerFunc {
	if nonceStore == nil {
		nonceStore = NewMemoryNonceStore()
	}
	if allowedSkew <= 0 {
		allowedSkew = 5 * time.Minute
	}
	return func(ctx *gin.Context) {
		appID := strings.TrimSpace(ctx.GetHeader("X-App-Id"))
		timestamp := strings.TrimSpace(ctx.GetHeader("X-Timestamp"))
		nonce := strings.TrimSpace(ctx.GetHeader("X-Nonce"))
		signature := strings.TrimSpace(ctx.GetHeader("X-Signature"))
		if appID == "" || timestamp == "" || nonce == "" || signature == "" {
			writeAuthError(ctx, "missing authentication headers")
			return
		}

		credential, ok := credentials[appID]
		if !ok || !credential.Enabled || credential.AppSecret == "" {
			writeAuthError(ctx, "invalid app credentials")
			return
		}
		if !validTimestamp(timestamp, allowedSkew) {
			writeAuthError(ctx, "expired authentication timestamp")
			return
		}

		body, err := readAndRestoreBody(ctx)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "read request body failed"})
			return
		}
		if !validSignature(ctx.Request.Method, ctx.Request.URL.Path, appID, timestamp, nonce, body, credential.AppSecret, signature) {
			writeAuthError(ctx, "invalid request signature")
			return
		}
		if !nonceStore.Use(appID, nonce, allowedSkew) {
			writeAuthError(ctx, "replayed request nonce")
			return
		}

		ctx.Set(authAppIDKey, appID)
		ctx.Next()
	}
}

func AuthAppID(ctx *gin.Context) string {
	if value, ok := ctx.Get(authAppIDKey); ok {
		if appID, ok := value.(string); ok {
			return appID
		}
	}
	return ""
}

func readAndRestoreBody(ctx *gin.Context) ([]byte, error) {
	if ctx.Request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return nil, err
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func validTimestamp(value string, allowedSkew time.Duration) bool {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return false
	}
	requestTime := time.Unix(seconds, 0)
	now := time.Now()
	return requestTime.After(now.Add(-allowedSkew)) && requestTime.Before(now.Add(allowedSkew))
}

func validSignature(method, path, appID, timestamp, nonce string, body []byte, appSecret, signature string) bool {
	provided, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	expected := requestSignature(method, path, appID, timestamp, nonce, body, appSecret)
	return hmac.Equal(provided, expected)
}

func requestSignature(method, path, appID, timestamp, nonce string, body []byte, appSecret string) []byte {
	bodyHash := sha256.Sum256(body)
	signingString := strings.Join([]string{
		method,
		path,
		appID,
		timestamp,
		nonce,
		hex.EncodeToString(bodyHash[:]),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(signingString))
	return mac.Sum(nil)
}

func writeAuthError(ctx *gin.Context, message string) {
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": message})
}
