package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"neonstack/gateway/internal/store"
)

var (
	ErrInvalidCredentials = errors.New("invalid name or password")
	ErrNameTaken          = errors.New("이미 사용 중인 이름입니다")
	ErrInvalidInput       = errors.New("invalid input")
)

type Service struct {
	store *store.Store
}

func New(st *store.Store) *Service { return &Service{store: st} }

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Service) Register(ctx context.Context, name, password string) (*store.User, string, error) {
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 16 {
		return nil, "", ErrInvalidInput
	}
	if len(password) < 4 {
		return nil, "", ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}
	if err := s.store.CreateUser(ctx, randomHex(8), name, string(hash)); err != nil {
		return nil, "", ErrNameTaken
	}
	return s.Login(ctx, name, password)
}

func (s *Service) Login(ctx context.Context, name, password string) (*store.User, string, error) {
	u, err := s.store.UserByName(ctx, name)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) != nil {
		return nil, "", ErrInvalidCredentials
	}
	token := randomHex(24)
	if err := s.store.CreateSession(ctx, token, u.ID); err != nil {
		return nil, "", err
	}
	return u, token, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.DeleteSession(ctx, token)
}

func (s *Service) UserFromToken(ctx context.Context, token string) (*store.User, error) {
	return s.store.UserByToken(ctx, token)
}

type userCtxKey struct{}

func WithUser(ctx context.Context, u *store.User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, u)
}

func UserFrom(ctx context.Context) *store.User {
	u, _ := ctx.Value(userCtxKey{}).(*store.User)
	return u
}

// RequireAuth wraps a handler, requiring a valid "Authorization: Bearer <token>" header.
func (s *Service) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := BearerToken(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		u, err := s.store.UserByToken(r.Context(), token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r.WithContext(WithUser(r.Context(), u)))
	}
}

func BearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}
