package httpx

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/forge-platform/forge/internal/platform/apperr"
)

// Verifier validates a bearer token and returns the authenticated principal.
// Two strategies implement it: HMAC (local development) and JWKS/RS256 (OIDC).
type Verifier interface {
	Verify(ctx context.Context, token string) (Principal, error)
}

type claims struct {
	jwt.RegisteredClaims
	Email  string   `json:"email"`
	Groups []string `json:"groups"`
}

func principalFromClaims(c claims) Principal {
	return Principal{Subject: c.Subject, Email: c.Email, Groups: c.Groups}
}

// HMACVerifier is for local development only and is rejected in production by
// configuration validation.
type HMACVerifier struct{ secret []byte }

func NewHMACVerifier(secret string) *HMACVerifier { return &HMACVerifier{secret: []byte(secret)} }

func (v *HMACVerifier) Verify(_ context.Context, token string) (Principal, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return v.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return Principal{}, apperr.Unauthorized("INVALID_TOKEN", "token verification failed")
	}
	return principalFromClaims(c), nil
}

// JWKSVerifier validates RS256 tokens against a cached JWKS document, refreshing
// on an unknown key id to handle key rotation.
type JWKSVerifier struct {
	url      string
	issuer   string
	audience string
	client   *http.Client
	ttl      time.Duration

	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

func NewJWKSVerifier(url, issuer, audience string) *JWKSVerifier {
	return &JWKSVerifier{
		url:      url,
		issuer:   issuer,
		audience: audience,
		client:   &http.Client{Timeout: 5 * time.Second},
		ttl:      10 * time.Minute,
		keys:     map[string]*rsa.PublicKey{},
	}
}

func (v *JWKSVerifier) Verify(ctx context.Context, token string) (Principal, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return v.keyForKID(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Principal{}, apperr.Unauthorized("INVALID_TOKEN", "token verification failed")
	}
	return principalFromClaims(c), nil
}

func (v *JWKSVerifier) keyForKID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	fresh := time.Since(v.fetched) < v.ttl
	v.mu.RUnlock()
	if ok && fresh {
		return key, nil
	}
	if err := v.refresh(ctx); err != nil {
		return nil, err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	if key, ok := v.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("signing key %q not found in JWKS", kid)
}

func (v *JWKSVerifier) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned status %d", resp.StatusCode)
	}
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	parsed := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			return err
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			return err
		}
		parsed[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(new(big.Int).SetBytes(eBytes).Int64()),
		}
	}
	v.mu.Lock()
	v.keys = parsed
	v.fetched = time.Now()
	v.mu.Unlock()
	return nil
}

// Authenticate enforces a valid bearer token and injects the principal.
func Authenticate(v Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("Authorization")
			if !strings.HasPrefix(raw, "Bearer ") {
				Error(w, r, apperr.Unauthorized("MISSING_TOKEN", "a bearer token is required"))
				return
			}
			principal, err := v.Verify(r.Context(), strings.TrimPrefix(raw, "Bearer "))
			if err != nil {
				Error(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), ctxPrincipal, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
