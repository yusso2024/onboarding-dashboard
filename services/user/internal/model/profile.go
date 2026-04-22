package model

import "time"

// Profile represents a user's onboarding data.
//
// WHY does this have UserID but not email/password?
// The User Service doesn't own authentication data.
// UserID is a foreign reference — it came from the JWT,
// which was issued by the Auth Service. This is how
// microservices reference each other's data without
// sharing databases: through IDs passed in tokens/headers.
type Profile struct {
	ID              int       `json:"id"`
	UserID          int       `json:"user_id"`
	DisplayName     string    `json:"display_name"`
	Role            string    `json:"role"`
	OnboardingStep  int       `json:"onboarding_step"`
	OnboardingDone  bool      `json:"onboarding_done"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateProfileRequest is sent when a new user completes registration
// and starts onboarding.
type CreateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// UpdateProfileRequest allows partial updates to profile data.
type UpdateProfileRequest struct {
	DisplayName    *string `json:"display_name,omitempty"`
	Role           *string `json:"role,omitempty"`
	OnboardingStep *int    `json:"onboarding_step,omitempty"`
}

// WHY pointer fields (*string, *int) in UpdateProfileRequest?
// Without pointers, you can't distinguish between:
//   - "user sent display_name as empty string" (intentional clear)
//   - "user didn't send display_name at all" (don't touch it)
// A nil pointer means "not provided." A non-nil pointer means
// "update to this value." This is the "partial update" pattern —
// critical in APIs where you don't want to require all fields
// on every update.
