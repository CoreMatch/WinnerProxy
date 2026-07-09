// Package hrpauth is a thin HTTP client for the HRPAuth (HA) backend.
//
// It speaks Yggdrasil on the front (hasJoined, profile, batch query) and
// exposes a single M.T.-authenticated /register helper for proxy-side
// registration. All identity data lives in HRPAuth; this client is
// stateless and only translates requests.
package hrpauth

// PlayerProfile mirrors the Yggdrasil session/profile response shape.
type PlayerProfile struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Properties []PlayerProperty `json:"properties"`
}

// PlayerProperty is a single signed property (e.g. textures).
type PlayerProperty struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

// RegisterRequest is the body of POST /register when called via the
// Manage Token (M.T.) path. Per HA-ROADMAP §3.2, M.T. is sent in the
// remember_token field; mojang_uuid is optional; email is auto-filled
// with a placeholder by HA when empty.
type RegisterRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Email         string `json:"email"`
	MojangUUID    string `json:"mojang_uuid,omitempty"`
	RememberToken string `json:"remember_token,omitempty"`
}

// RegisterResponse is what HA returns on a successful POST /register.
// CBH is populated only when a brand-new proxy-registered account is
// created (cbh=0); idempotent and bind paths leave it zero.
type RegisterResponse struct {
	Success   bool   `json:"success"`
	UID       int64  `json:"uid"`
	Message   string `json:"message"`
	ProfileID string `json:"profile_id"`
	CBH       int    `json:"cbh,omitempty"`
}
