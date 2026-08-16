package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"neonstack/gateway/internal/store"
)

var (
	ErrInvalidCredentials  = errors.New("invalid name or password")
	ErrNameTaken           = errors.New("이미 사용 중인 이름입니다")
	ErrInvalidInput        = errors.New("invalid input")
	ErrGoogleNotConfigured = errors.New("구글 로그인이 설정되지 않았습니다")
	ErrGoogleTokenInvalid  = errors.New("구글 로그인 인증에 실패했습니다")
)

type Service struct {
	store          *store.Store
	googleClientID string
}

func New(st *store.Store, googleClientID string) *Service {
	return &Service{store: st, googleClientID: googleClientID}
}

// GoogleClientID returns the configured Google OAuth client id (empty = disabled).
func (s *Service) GoogleClientID() string { return s.googleClientID }

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
	if u.Password == "" || bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) != nil {
		return nil, "", ErrInvalidCredentials
	}
	return s.issueSession(ctx, u)
}

// issueSession — 기존/신규 사용자에게 세션 토큰을 발급한다.
func (s *Service) issueSession(ctx context.Context, u *store.User) (*store.User, string, error) {
	token := randomHex(24)
	if err := s.store.CreateSession(ctx, token, u.ID); err != nil {
		return nil, "", err
	}
	return u, token, nil
}

// uniqueName — 구글 프로필 이름이 이미 사용 중이면 접미사를 붙여 고유 이름을 만든다.
func (s *Service) uniqueName(ctx context.Context, base, sub string) string {
	cand := strings.TrimSpace(base)
	if cand == "" {
		cand = "player"
	}
	if utf8.RuneCountInString(cand) > 16 {
		// rune 기준 16자로 자른다
		runes := []rune(cand)
		cand = string(runes[:16])
	}
	for i := 0; ; i++ {
		name := cand
		if i > 0 {
			suffix := fmt.Sprintf("%d", i)
			runes := []rune(cand)
			cut := 16 - len(suffix)
			if cut < 2 {
				cut = 2
			}
			if len(runes) > cut {
				runes = runes[:cut]
			}
			name = string(runes) + suffix
		}
		ok, err := s.store.NameAvailable(ctx, name)
		if err == nil && ok {
			return name
		}
		if i > 50 {
			return "user-" + sub
		}
	}
}

// googleTokenInfo — tokeninfo 엔드포인트가 반환하는 ID 토큰 정보.
type googleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Aud           string `json:"aud"`
}

// verifyGoogleToken — Google ID 토큰(JWT)을 tokeninfo 엔드포인트로 검증한다.
// 서명·만료는 Google이 검증하며, 여기서는 aud(발급 대상)와 email_verified를 확인한다.
func verifyGoogleToken(ctx context.Context, clientID, idToken string) (*googleTokenInfo, error) {
	if idToken == "" {
		return nil, ErrGoogleTokenInvalid
	}
	endpoint := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrGoogleTokenInvalid
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, ErrGoogleTokenInvalid
	}
	var info googleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, ErrGoogleTokenInvalid
	}
	if info.Aud != clientID {
		return nil, ErrGoogleTokenInvalid
	}
	if info.EmailVerified != "true" {
		return nil, ErrGoogleTokenInvalid
	}
	return &info, nil
}

// GoogleLogin — Google ID 토큰으로 로그인/가입한다.
// 이미 구글 계정이 연결된 사용자, 같은 이메일의 기존 사용자(링크), 신규 사용자(생성) 순으로 처리.
func (s *Service) GoogleLogin(ctx context.Context, idToken string) (*store.User, string, error) {
	if s.googleClientID == "" {
		return nil, "", ErrGoogleNotConfigured
	}
	info, err := verifyGoogleToken(ctx, s.googleClientID, idToken)
	if err != nil {
		return nil, "", err
	}

	// 1) 이미 연결된 구글 계정
	if u, err := s.store.FindUserByGoogleSub(ctx, info.Sub); err == nil {
		return s.issueSession(ctx, u)
	}

	// 2) 같은 이메일의 기존 사용자 → 구글 계정 연결
	if info.Email != "" {
		if u, err := s.store.FindUserByEmail(ctx, info.Email); err == nil {
			_ = s.store.LinkGoogle(ctx, u.ID, info.Sub, info.Email)
			return s.issueSession(ctx, u)
		}
	}

	// 3) 신규 사용자
	name := s.uniqueName(ctx, info.Name, info.Sub)
	u := &store.User{ID: randomHex(8), Name: name, Email: info.Email, GoogleSub: info.Sub}
	if err := s.store.CreateGoogleUser(ctx, u.ID, u.Name, u.Email, u.GoogleSub); err != nil {
		return nil, "", err
	}
	return s.issueSession(ctx, u)
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
