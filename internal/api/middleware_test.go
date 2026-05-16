package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

const testBotToken = "test-bot-token"

// buildInitData constructs a valid signed initData string using the given params and bot token.
func buildInitData(t *testing.T, userID int64, username string, authDate int64, botToken string) string {
	t.Helper()

	userJSON, _ := json.Marshal(map[string]any{"id": userID, "username": username})

	params := url.Values{}
	params.Set("auth_date", fmt.Sprintf("%d", authDate))
	params.Set("user", string(userJSON))

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(k + "=" + params.Get(k))
	}

	mac := hmac.New(sha256.New, []byte("WebAppData"))
	mac.Write([]byte(botToken))
	secretKey := mac.Sum(nil)

	mac2 := hmac.New(sha256.New, secretKey)
	mac2.Write([]byte(sb.String()))
	hash := hex.EncodeToString(mac2.Sum(nil))

	params.Set("hash", hash)
	return params.Encode()
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	username := getUsername(r)
	fmt.Fprintf(w, "%d:%s", userID, username)
}

func TestInitDataMiddleware_ValidRequest(t *testing.T) {
	authDate := time.Now().Unix()
	initData := buildInitData(t, 123456, "testuser", authDate, testBotToken)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "tma "+initData)
	w := httptest.NewRecorder()

	handler := InitDataMiddleware(testBotToken, 10*time.Minute)(http.HandlerFunc(okHandler))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "123456:testuser") {
		t.Errorf("Expected userID and username in context, got %s", w.Body.String())
	}
}

func TestInitDataMiddleware_MissingAuthHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler := InitDataMiddleware(testBotToken, 10*time.Minute)(http.HandlerFunc(okHandler))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestInitDataMiddleware_WrongAuthScheme(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	w := httptest.NewRecorder()

	handler := InitDataMiddleware(testBotToken, 10*time.Minute)(http.HandlerFunc(okHandler))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestInitDataMiddleware_InvalidHash(t *testing.T) {
	authDate := time.Now().Unix()
	initData := buildInitData(t, 123456, "testuser", authDate, testBotToken)

	// Tamper the hash
	params, _ := url.ParseQuery(initData)
	params.Set("hash", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	initData = params.Encode()

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "tma "+initData)
	w := httptest.NewRecorder()

	handler := InitDataMiddleware(testBotToken, 10*time.Minute)(http.HandlerFunc(okHandler))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestInitDataMiddleware_WrongBotToken(t *testing.T) {
	authDate := time.Now().Unix()
	initData := buildInitData(t, 123456, "testuser", authDate, "wrong-token")

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "tma "+initData)
	w := httptest.NewRecorder()

	handler := InitDataMiddleware(testBotToken, 10*time.Minute)(http.HandlerFunc(okHandler))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestInitDataMiddleware_ExpiredAuthDate(t *testing.T) {
	authDate := time.Now().Add(-20 * time.Minute).Unix()
	initData := buildInitData(t, 123456, "testuser", authDate, testBotToken)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "tma "+initData)
	w := httptest.NewRecorder()

	handler := InitDataMiddleware(testBotToken, 10*time.Minute)(http.HandlerFunc(okHandler))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestInitDataMiddleware_TamperedData(t *testing.T) {
	authDate := time.Now().Unix()
	initData := buildInitData(t, 123456, "testuser", authDate, testBotToken)

	// Tamper userID after signing — signature should no longer match
	params, _ := url.ParseQuery(initData)
	userJSON, _ := json.Marshal(map[string]any{"id": 999999, "username": "hacker"})
	params.Set("user", string(userJSON))
	initData = params.Encode()

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "tma "+initData)
	w := httptest.NewRecorder()

	handler := InitDataMiddleware(testBotToken, 10*time.Minute)(http.HandlerFunc(okHandler))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for tampered data, got %d", w.Code)
	}
}

func TestValidateInitData_MissingHash(t *testing.T) {
	params := url.Values{}
	params.Set("auth_date", fmt.Sprintf("%d", time.Now().Unix()))
	params.Set("user", `{"id":1,"username":"u"}`)

	_, _, err := validateInitData(params.Encode(), testBotToken, 10*time.Minute)
	if err == nil {
		t.Error("Expected error for missing hash")
	}
}

func TestValidateInitData_InvalidAuthDate(t *testing.T) {
	params := url.Values{}
	params.Set("auth_date", "notanumber")
	params.Set("user", `{"id":1,"username":"u"}`)
	params.Set("hash", "abc")

	_, _, err := validateInitData(params.Encode(), testBotToken, 10*time.Minute)
	if err == nil {
		t.Error("Expected error for invalid auth_date")
	}
}

func TestCORSMiddleware(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header Access-Control-Allow-Origin: *")
	}
}

func TestCORSMiddleware_Options(t *testing.T) {
	req := httptest.NewRequest("OPTIONS", "/", nil)
	w := httptest.NewRecorder()

	CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected 204 for OPTIONS, got %d", w.Code)
	}
}
