package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type contextKey string

const (
	contextUserID   contextKey = "userID"
	contextUsername contextKey = "username"
)

func validateInitData(initDataRaw, botToken string, ttl time.Duration) (int64, string, error) {
	params, err := url.ParseQuery(initDataRaw)
	if err != nil {
		return 0, "", err
	}

	hash := params.Get("hash")
	if hash == "" {
		return 0, "", errUnauthorized("missing hash")
	}
	params.Del("hash")

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
	computed := hex.EncodeToString(mac2.Sum(nil))

	if !hmac.Equal([]byte(computed), []byte(hash)) {
		return 0, "", errUnauthorized("invalid hash")
	}

	authDate, err := strconv.ParseInt(params.Get("auth_date"), 10, 64)
	if err != nil {
		return 0, "", errUnauthorized("invalid auth_date")
	}
	if time.Since(time.Unix(authDate, 0)) > ttl {
		return 0, "", errUnauthorized("initData expired")
	}

	var user struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal([]byte(params.Get("user")), &user); err != nil {
		return 0, "", errUnauthorized("invalid user field")
	}

	return user.ID, user.Username, nil
}

type authError struct{ msg string }

func (e authError) Error() string { return e.msg }

func errUnauthorized(msg string) authError { return authError{msg} }

func InitDataMiddleware(botToken string, ttl time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "tma ") {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			initDataRaw := strings.TrimPrefix(authHeader, "tma ")

			userID, username, err := validateInitData(initDataRaw, botToken, ttl)
			if err != nil {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), contextUserID, userID)
			ctx = context.WithValue(ctx, contextUsername, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getUserID(r *http.Request) int64 {
	v, _ := r.Context().Value(contextUserID).(int64)
	return v
}

func getUsername(r *http.Request) string {
	v, _ := r.Context().Value(contextUsername).(string)
	return v
}
