package auth

import "testing"

func TestEmailFromJWT(t *testing.T) {
	// This is a real JWT structure (payload only matters).
	// payload: {"email": "test@example.com", "sub": "12345"}
	// base64url of that: eyJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJzdWIiOiIxMjM0NSJ9
	token := "eyJhbGciOiJSUzI1NiJ9.eyJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJzdWIiOiIxMjM0NSJ9.signature"

	email := emailFromJWT(token)
	if email != "test@example.com" {
		t.Errorf("emailFromJWT() = %q, want %q", email, "test@example.com")
	}
}

func TestEmailFromJWTInvalid(t *testing.T) {
	tests := []string{
		"",
		"not-a-jwt",
		"header.!!!invalid-base64!!!.sig",
		"header..sig",
	}
	for _, tt := range tests {
		email := emailFromJWT(tt)
		if email != "" {
			t.Errorf("emailFromJWT(%q) = %q, want empty", tt, email)
		}
	}
}
