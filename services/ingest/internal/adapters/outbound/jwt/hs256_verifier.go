package jwt

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"goanalytics/services/ingest/internal/domain/token"
)

// Errores devueltos por el verificador JWT HS256.
var (
	ErrEmptyToken        = errors.New("jwt vacio")
	ErrMalformedToken    = errors.New("jwt mal formado")
	ErrUnsupportedAlg    = errors.New("algoritmo jwt no soportado")
	ErrInvalidSignature  = errors.New("firma jwt invalida")
	ErrInvalidJWTClaims  = errors.New("claims jwt invalidos")
	ErrTokenLifetimeHigh = errors.New("vida util del jwt excedida")
)

// HS256Verifier valida tokens JWT firmados con HMAC SHA-256.
//
// Contiene secreto compartido, issuer, audience y vida maxima opcional del
// token. Implementa el puerto EventTokenVerifier sin exponer detalles de JWT a
// la capa de aplicacion.
type HS256Verifier struct {
	secret           []byte
	issuer           string
	audience         string
	maxTokenLifetime time.Duration
}

// NewHS256Verifier crea un verificador JWT HS256.
//
// Recibe secreto compartido, issuer esperado, audience esperada y vida maxima
// opcional del token. Devuelve error cuando falta un valor obligatorio.
func NewHS256Verifier(secret string, issuer string, audience string, maxTokenLifetime time.Duration) (*HS256Verifier, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("%w: secreto requerido", ErrInvalidJWTClaims)
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, fmt.Errorf("%w: issuer requerido", ErrInvalidJWTClaims)
	}
	if strings.TrimSpace(audience) == "" {
		return nil, fmt.Errorf("%w: audience requerida", ErrInvalidJWTClaims)
	}
	return &HS256Verifier{
		secret:           []byte(secret),
		issuer:           issuer,
		audience:         audience,
		maxTokenLifetime: maxTokenLifetime,
	}, nil
}

// Verify valida firma, algoritmo y claims del JWT de tracking.
//
// Recibe contexto y token compacto sin prefijo Bearer. Devuelve
// token.TrackingClaims cuando el JWT cumple el contrato. Devuelve error cuando
// el token falta, esta mal formado, usa otro algoritmo, tiene firma invalida o
// claims incompatibles.
func (verifier *HS256Verifier) Verify(ctx context.Context, rawToken string) (token.TrackingClaims, error) {
	_ = ctx
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return token.TrackingClaims{}, ErrEmptyToken
	}

	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return token.TrackingClaims{}, ErrMalformedToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return token.TrackingClaims{}, fmt.Errorf("%w: header", ErrMalformedToken)
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return token.TrackingClaims{}, fmt.Errorf("%w: payload", ErrMalformedToken)
	}

	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return token.TrackingClaims{}, fmt.Errorf("%w: header json", ErrMalformedToken)
	}
	if header.Algorithm != "HS256" || (header.Type != "" && header.Type != "JWT") {
		return token.TrackingClaims{}, ErrUnsupportedAlg
	}
	if !verifier.signatureMatches(parts[0], parts[1], parts[2]) {
		return token.TrackingClaims{}, ErrInvalidSignature
	}

	var claims jwtClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return token.TrackingClaims{}, fmt.Errorf("%w: payload json", ErrMalformedToken)
	}
	return verifier.toTrackingClaims(claims)
}

// signatureMatches compara la firma recibida contra la firma HMAC esperada.
//
// Recibe header y payload codificados junto con la firma codificada. Devuelve
// true solo si la comparacion constante coincide.
func (verifier *HS256Verifier) signatureMatches(encodedHeader string, encodedPayload string, encodedSignature string) bool {
	mac := hmac.New(sha256.New, verifier.secret)
	mac.Write([]byte(encodedHeader + "." + encodedPayload))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(encodedSignature)
	if err != nil {
		return false
	}
	return hmac.Equal(actual, expected)
}

// toTrackingClaims convierte claims JWT genericos al DTO de dominio.
//
// Recibe claims ya parseados y valida issuer, audience, timestamps y vida
// maxima opcional. Devuelve token.TrackingClaims o un error de contrato.
func (verifier *HS256Verifier) toTrackingClaims(claims jwtClaims) (token.TrackingClaims, error) {
	if claims.Issuer != verifier.issuer {
		return token.TrackingClaims{}, fmt.Errorf("%w: issuer", ErrInvalidJWTClaims)
	}
	if !claims.Audience.Contains(verifier.audience) {
		return token.TrackingClaims{}, fmt.Errorf("%w: audience", ErrInvalidJWTClaims)
	}
	issuedAt, err := unixTime(claims.IssuedAt)
	if err != nil {
		return token.TrackingClaims{}, fmt.Errorf("%w: iat", ErrInvalidJWTClaims)
	}
	notBefore, err := unixTime(claims.NotBefore)
	if err != nil {
		return token.TrackingClaims{}, fmt.Errorf("%w: nbf", ErrInvalidJWTClaims)
	}
	expiresAt, err := unixTime(claims.ExpiresAt)
	if err != nil {
		return token.TrackingClaims{}, fmt.Errorf("%w: exp", ErrInvalidJWTClaims)
	}
	if verifier.maxTokenLifetime > 0 && expiresAt.Sub(issuedAt) > verifier.maxTokenLifetime {
		return token.TrackingClaims{}, ErrTokenLifetimeHigh
	}

	return token.TrackingClaims{
		Issuer:       claims.Issuer,
		Audience:     verifier.audience,
		SiteCode:     claims.SiteCode,
		Environment:  claims.Environment,
		TokenVersion: claims.TokenVersion,
		IssuedAt:     issuedAt,
		NotBefore:    notBefore,
		ExpiresAt:    expiresAt,
		JWTID:        claims.JWTID,
		TenantHint:   claims.TenantHint,
		SiteHint:     claims.SiteHint,
	}, nil
}

// jwtHeader representa los campos minimos del header JWT.
type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

// jwtClaims representa el payload firmado del token de tracking.
type jwtClaims struct {
	Issuer       string       `json:"iss"`
	Audience     audienceList `json:"aud"`
	SiteCode     string       `json:"site_code"`
	Environment  string       `json:"env"`
	TokenVersion int          `json:"token_version"`
	IssuedAt     json.Number  `json:"iat"`
	NotBefore    json.Number  `json:"nbf"`
	ExpiresAt    json.Number  `json:"exp"`
	JWTID        string       `json:"jti"`
	TenantHint   string       `json:"tenant_hint"`
	SiteHint     string       `json:"site_hint"`
}

// audienceList acepta aud como string o como lista de strings.
type audienceList []string

// UnmarshalJSON parsea el claim aud en sus dos formas habituales.
//
// Recibe JSON crudo y carga la lista de audiencias. Devuelve error cuando el
// valor no es string ni arreglo de strings.
func (audiences *audienceList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*audiences = []string{single}
		return nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	*audiences = list
	return nil
}

// Contains informa si la audiencia esperada esta presente.
//
// Recibe una audiencia esperada y devuelve true cuando existe en la lista
// parseada del token.
func (audiences audienceList) Contains(expected string) bool {
	for _, value := range audiences {
		if value == expected {
			return true
		}
	}
	return false
}

// unixTime convierte un json.Number de segundos Unix a time.Time UTC.
//
// Recibe el numero del claim temporal y devuelve error cuando falta o no puede
// interpretarse como entero o flotante.
func unixTime(value json.Number) (time.Time, error) {
	if strings.TrimSpace(value.String()) == "" {
		return time.Time{}, ErrInvalidJWTClaims
	}
	seconds, err := strconv.ParseInt(value.String(), 10, 64)
	if err != nil {
		floatSeconds, floatErr := strconv.ParseFloat(value.String(), 64)
		if floatErr != nil {
			return time.Time{}, err
		}
		seconds = int64(floatSeconds)
	}
	return time.Unix(seconds, 0).UTC(), nil
}
