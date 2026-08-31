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

// RegisterRequest is the body of POST /register.
type RegisterRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Email      string `json:"email"`
	MojangUUID string `json:"mojang_uuid,omitempty"`
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

// DeclareEmailRequest is the body for POST /user/declare-email.
type DeclareEmailRequest struct {
	Email      string `json:"email"`
	PlayerName string `json:"playername"`
}

// EnableMojangBindRequest is the body for POST /user/mojang-bind-enable.
type EnableMojangBindRequest struct {
	UID   string `json:"uid,omitempty"`
	Email string `json:"email,omitempty"`
}

// PresenceRequest is the body of the microservice presence handshake
// (POST /services/presence, the "bonjour" handshake). Only the fields
// WinnerProxy uses are modeled; optional contract fields (scope,
// sdk_url, security_level, interacts_with) are omitted and stay unset.
type PresenceRequest struct {
	Name string `json:"name"`
	// TTLSeconds is the self-declared lifetime in seconds; <=0 or
	// omitted means the record never expires.
	TTLSeconds int `json:"ttl_seconds"`
}

// CommonResponse is a generic HA response shape.
type CommonResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
