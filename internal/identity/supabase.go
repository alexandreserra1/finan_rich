package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrConfiguration = errors.New("o serviço de contas não está configurado corretamente")
	ErrInput         = errors.New("informe um e-mail válido e uma senha de até 72 bytes; no cadastro, use pelo menos 15 caracteres")
	ErrCredentials   = errors.New("não foi possível entrar; confira os dados e a confirmação do e-mail")
	ErrUnavailable   = errors.New("o serviço de contas não respondeu como esperado; tente novamente mais tarde")
)

const requestTimeout = 10 * time.Second
const maxResponseBytes = 64 << 10

var providerID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Identity struct {
	ID    string
	Email string
}

type Supabase struct {
	base   string
	key    string
	client http.Client
}

func NewSupabase(projectURL, publishableKey string, client *http.Client) (*Supabase, error) {
	endpoint, err := url.Parse(projectURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil ||
		endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" ||
		(endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawPath != "" {
		return nil, ErrConfiguration
	}
	if !strings.HasPrefix(publishableKey, "sb_publishable_") || len(publishableKey) <= len("sb_publishable_") || len(publishableKey) > 1024 {
		return nil, ErrConfiguration
	}
	for _, character := range publishableKey {
		if character < 33 || character > 126 {
			return nil, ErrConfiguration
		}
	}
	var safeClient http.Client
	if client != nil {
		safeClient = *client
	}
	if safeClient.Timeout <= 0 || safeClient.Timeout > requestTimeout {
		safeClient.Timeout = requestTimeout
	}
	safeClient.Jar = nil
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Supabase{base: strings.TrimSuffix(endpoint.String(), "/") + "/auth/v1", key: publishableKey, client: safeClient}, nil
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type providerResponse struct {
	ID               string     `json:"id"`
	Email            string     `json:"email"`
	EmailConfirmedAt *time.Time `json:"email_confirmed_at"`
	AccessToken      string     `json:"access_token"`
	RefreshToken     string     `json:"refresh_token"`
	TokenType        string     `json:"token_type"`
	ErrorCode        string     `json:"error_code"`
}

func (adapter *Supabase) Signup(ctx context.Context, email, password string) error {
	input, err := validateCredentials(email, password, 15)
	if err != nil {
		return err
	}
	response, status, err := adapter.request(ctx, http.MethodPost, "/signup", input, "")
	if err != nil {
		return err
	}
	if (status == http.StatusBadRequest || status == http.StatusUnprocessableEntity) && response.ErrorCode == "user_already_exists" {
		return nil
	}
	if status != http.StatusOK || response.ErrorCode != "" || !validProviderID(response.ID) ||
		response.AccessToken != "" || response.RefreshToken != "" || response.EmailConfirmedAt != nil {
		return ErrUnavailable
	}
	return nil
}

func (adapter *Supabase) Authenticate(ctx context.Context, email, password string) (Identity, error) {
	input, err := validateCredentials(email, password, 1)
	if err != nil {
		return Identity{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	response, status, err := adapter.request(ctx, http.MethodPost, "/token?grant_type=password", input, "")
	if err != nil {
		return Identity{}, err
	}
	if (status == http.StatusBadRequest || status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusUnprocessableEntity) &&
		(response.ErrorCode == "invalid_credentials" || response.ErrorCode == "email_not_confirmed") {
		return Identity{}, ErrCredentials
	}
	if status != http.StatusOK || response.ErrorCode != "" || response.AccessToken == "" ||
		len(response.AccessToken) > 16<<10 || !strings.EqualFold(response.TokenType, "bearer") ||
		strings.ContainsAny(response.AccessToken, " \t\r\n\x00") {
		return Identity{}, ErrUnavailable
	}
	user, status, err := adapter.request(ctx, http.MethodGet, "/user", nil, response.AccessToken)
	if err != nil {
		return Identity{}, err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return Identity{}, ErrCredentials
	}
	if status != http.StatusOK || user.ErrorCode != "" || !validProviderID(user.ID) || !validEmail(user.Email) {
		return Identity{}, ErrUnavailable
	}
	if user.EmailConfirmedAt == nil || user.EmailConfirmedAt.IsZero() {
		return Identity{}, ErrCredentials
	}
	return Identity{ID: user.ID, Email: user.Email}, nil
}

func (adapter *Supabase) request(ctx context.Context, method, path string, input any, token string) (providerResponse, int, error) {
	var body io.Reader
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return providerResponse{}, 0, ErrInput
		}
		body = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, adapter.base+path, body)
	if err != nil {
		return providerResponse{}, 0, ErrUnavailable
	}
	request.Header.Set("apikey", adapter.key)
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := adapter.client.Do(request)
	if err != nil {
		return providerResponse{}, 0, ErrUnavailable
	}
	defer response.Body.Close()
	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return providerResponse{}, 0, ErrUnavailable
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(payload) > maxResponseBytes {
		return providerResponse{}, 0, ErrUnavailable
	}
	var result providerResponse
	if json.Unmarshal(payload, &result) != nil {
		return providerResponse{}, 0, ErrUnavailable
	}
	return result, response.StatusCode, nil
}

func validateCredentials(email, password string, minimum int) (credentials, error) {
	if strings.ContainsFunc(email, unicode.IsControl) || !utf8.ValidString(password) ||
		strings.ContainsRune(password, '\x00') || len(password) > 72 || utf8.RuneCountInString(password) < minimum {
		return credentials{}, ErrInput
	}
	email = strings.TrimSpace(email)
	if !validEmail(email) {
		return credentials{}, ErrInput
	}
	return credentials{Email: email, Password: password}, nil
}

func validEmail(email string) bool {
	if len(email) > 254 || !utf8.ValidString(email) || strings.ContainsFunc(email, unicode.IsControl) {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && address.Name == ""
}

func validProviderID(value string) bool {
	return providerID.MatchString(value) && value != "00000000-0000-0000-0000-000000000000"
}
