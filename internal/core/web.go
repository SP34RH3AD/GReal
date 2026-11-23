package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// httpClient is the shared HTTP client for web-based enumeration modes.
var httpClient = &http.Client{Timeout: 20 * time.Second}

const userAgent = "Mozilla/5.0 (compatible; GReal/1.3)"

// Endpoint bases are variables so tests can point them at a local server.
var (
	o365CredTypeURL = "https://login.microsoftonline.com/common/GetCredentialType"
	o365RealmURL    = "https://login.microsoftonline.com/getuserrealm.srf"
)

// --- Microsoft 365 / Azure AD ---

type credTypeResp struct {
	IfExistsResult int `json:"IfExistsResult"`
	ThrottleStatus int `json:"ThrottleStatus"`
}

// o365AccountExists reports whether an O365 / Azure AD account exists, using the
// unauthenticated GetCredentialType endpoint. IfExistsResult == 0 means the
// account exists; a non-zero ThrottleStatus means the result is unreliable.
func o365AccountExists(email string) (bool, error) {
	body, _ := json.Marshal(map[string]string{"Username": email})
	req, err := http.NewRequest(http.MethodPost,
		o365CredTypeURL,
		bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, err
	}

	var ct credTypeResp
	if err := json.Unmarshal(data, &ct); err != nil {
		return false, err
	}
	if ct.ThrottleStatus != 0 {
		return false, fmt.Errorf("throttled by endpoint")
	}
	return ct.IfExistsResult == 0, nil
}

type userRealm struct {
	NameSpaceType string `json:"NameSpaceType"`
}

// isO365Domain reports whether a domain is backed by Microsoft 365 (Managed or
// Federated), via the getuserrealm endpoint.
func isO365Domain(domain string) bool {
	u := fmt.Sprintf("%s?login=test@%s&json=1", o365RealmURL, domain)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}
	var r userRealm
	if json.Unmarshal(data, &r) != nil {
		return false
	}
	t := strings.ToLower(r.NameSpaceType)
	return t == "managed" || t == "federated"
}

// o365MX returns the preferred Microsoft 365 MX host for a domain.
func o365MX(domain string) (string, error) {
	mx, err := lookupMX(domain)
	if err != nil {
		return "", err
	}
	for _, m := range mx {
		l := strings.ToLower(m)
		if strings.Contains(l, "protection.outlook.com") || strings.Contains(l, "mail.protection") {
			return l, nil
		}
	}
	if len(mx) > 0 {
		return mx[0], nil
	}
	return "", fmt.Errorf("no MX records for %s", domain)
}

// --- Google Workspace ---

// isGoogleDomain reports whether a domain's MX records point at Google.
func isGoogleDomain(domain string) bool {
	mx, _ := lookupMX(domain)
	for _, m := range mx {
		l := strings.ToLower(m)
		if strings.Contains(l, "google") || strings.Contains(l, "googlemail") {
			return true
		}
	}
	return false
}

// googleMX returns the preferred Google MX host for a domain.
func googleMX(domain string) (string, error) {
	mx, err := lookupMX(domain)
	if err != nil {
		return "", err
	}
	for _, m := range mx {
		if strings.Contains(strings.ToLower(m), "google") {
			return strings.ToLower(m), nil
		}
	}
	if len(mx) > 0 {
		return mx[0], nil
	}
	return "", fmt.Errorf("no MX records for %s", domain)
}
