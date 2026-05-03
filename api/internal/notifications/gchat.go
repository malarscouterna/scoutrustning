package notifications

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/crypto"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

// GChatNotifier sends broadcast card messages to Google Chat Spaces via the REST API.
// Auth: service account JSON with Domain-Wide Delegation, impersonating gchat_admin_email.
// To must be the space resource name, e.g. "spaces/AAAA123".
type GChatNotifier struct {
	Q *db.Queries
}

type serviceAccountKey struct {
	Type        string `json:"type"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

// GChatSpace is a Google Chat Space returned by the spaces.list API.
type GChatSpace struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

func gchatAccessToken(ctx context.Context, saJSON []byte, adminEmail string) (string, error) {
	var key serviceAccountKey
	if err := json.Unmarshal(saJSON, &key); err != nil {
		return "", fmt.Errorf("gchat: parse service account: %w", err)
	}

	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("gchat: invalid private key PEM")
	}
	raw, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("gchat: parse private key: %w", err)
	}
	rsaKey, ok := raw.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("gchat: expected RSA private key")
	}

	tokenURI := key.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   key.ClientEmail,
		"scope": "https://www.googleapis.com/auth/chat.bot",
		"aud":   tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	if adminEmail != "" {
		claims["sub"] = adminEmail
	}

	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := t.SignedString(rsaKey)
	if err != nil {
		return "", fmt.Errorf("gchat: sign JWT: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI,
		strings.NewReader(url.Values{
			"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
			"assertion":  {signed},
		}.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gchat: token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gchat: token exchange failed (%d): %s", resp.StatusCode, body)
	}

	var tr struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("gchat: decode token response: %w", err)
	}
	return tr.AccessToken, nil
}

func gchatPost(ctx context.Context, token, path string, body interface{}) error {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://chat.googleapis.com/v1/"+path,
		strings.NewReader(string(bodyJSON)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("gchat: POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gchat: POST %s failed (%d): %s", path, resp.StatusCode, rb)
	}
	return nil
}

// ListGChatSpaces lists the spaces accessible to the service account.
func ListGChatSpaces(ctx context.Context, saJSON []byte, adminEmail string) ([]GChatSpace, error) {
	token, err := gchatAccessToken(ctx, saJSON, adminEmail)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://chat.googleapis.com/v1/spaces?pageSize=100&filter=spaceType%3DSPACE", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gchat: list spaces: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gchat: list spaces failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		Spaces []GChatSpace `json:"spaces"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Spaces, nil
}

// AddBotToSpace posts a membership create and a welcome card to the space.
// Ignores 409 (already a member).
func AddBotToSpace(ctx context.Context, saJSON []byte, adminEmail, spaceName string) error {
	token, err := gchatAccessToken(ctx, saJSON, adminEmail)
	if err != nil {
		return err
	}

	// Add bot membership (ignore 409 already-member).
	memberBody := map[string]interface{}{
		"member": map[string]interface{}{
			"name": "users/app",
			"type": "BOT",
		},
	}
	bodyJSON, _ := json.Marshal(memberBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://chat.googleapis.com/v1/"+spaceName+"/members",
		strings.NewReader(string(bodyJSON)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("gchat: add bot member: %w", err)
	}
	resp.Body.Close()
	// 409 = already a member; acceptable.

	return gchatPost(ctx, token, spaceName+"/messages", map[string]interface{}{
		"text": "Google Chat-anslutning aktiverad! Boknings- och ärendenotiser kommer att skickas hit.",
	})
}

func (g *GChatNotifier) Send(ctx context.Context, msg Message) error {
	creds, err := g.Q.GetGchatCredentials(ctx, msg.GroupID)
	if err != nil || len(creds.GchatServiceAccountJsonEncrypted) == 0 {
		return fmt.Errorf("gchat: no credentials for group %s", msg.GroupID)
	}

	saJSON, err := crypto.Decrypt(creds.GchatServiceAccountJsonEncrypted)
	if err != nil {
		return fmt.Errorf("gchat: decrypt credentials: %w", err)
	}

	token, err := gchatAccessToken(ctx, saJSON, creds.GchatAdminEmail)
	if err != nil {
		return err
	}

	text := msg.Subject
	if msg.TextBody != "" {
		text += "\n\n" + msg.TextBody
	}

	chatMsg := map[string]interface{}{
		"text": text,
	}
	if msg.ThreadKey != "" {
		chatMsg["thread"] = map[string]interface{}{"threadKey": msg.ThreadKey}
	}

	// msg.To is the full space resource name, e.g. "spaces/AAAA123".
	return gchatPost(ctx, token, msg.To+"/messages?messageReplyOption=REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD", chatMsg)
}
