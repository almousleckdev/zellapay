package httpapi

import (
	"time"

	"github.com/zellapay/zellapay/internal/identity"
)

type acceptRequest struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type tokenRequest struct {
	Token string `json:"token"`
}

type emailRequest struct {
	Email string `json:"email"`
}

type resetRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type codeRequest struct {
	Code string `json:"code"`
}

type userJSON struct {
	ID            string `json:"id"`
	MerchantID    string `json:"merchant_id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Role          string `json:"role"`
	EmailVerified bool   `json:"email_verified"`
}

func userView(u identity.User) userJSON {
	return userJSON{
		ID: u.ID, MerchantID: u.MerchantID, Email: u.Email, Name: u.Name,
		Role: string(u.Role), EmailVerified: u.EmailVerifiedAt != nil,
	}
}

type sessionJSON struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	IP          string    `json:"ip"`
	UserAgent   string    `json:"user_agent"`
	MFAVerified bool      `json:"mfa_verified"`
	// MFAVerifiedAt is when the second factor was last proven, if ever.
	MFAVerifiedAt *time.Time `json:"mfa_verified_at"`
	Current       bool       `json:"current"`
}

func sessionView(s identity.Session, currentID string) sessionJSON {
	return sessionJSON{
		ID: s.ID, CreatedAt: s.CreatedAt, LastSeenAt: s.LastSeenAt,
		ExpiresAt: s.ExpiresAt, IP: s.Source.IP, UserAgent: s.Source.UserAgent,
		MFAVerified: s.MFAAt != nil, MFAVerifiedAt: s.MFAAt, Current: s.ID == currentID,
	}
}

type loginResponse struct {
	SessionToken string    `json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	MFA          string    `json:"mfa"`
}

type currentResponse struct {
	User    userJSON    `json:"user"`
	Session sessionJSON `json:"session"`
	MFA     string      `json:"mfa"`
}

type mfaResponse struct {
	RecoveryUsed bool `json:"recovery_code_used"`
	CodesLeft    int  `json:"recovery_codes_left"`
}

type enrolmentResponse struct {
	ManualKey string `json:"manual_key"`
	URI       string `json:"otpauth_uri"`
}

type recoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

type sessionsResponse struct {
	Data []sessionJSON `json:"data"`
}

type endedResponse struct {
	Ended int `json:"ended"`
}
