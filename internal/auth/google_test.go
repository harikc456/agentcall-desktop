package auth_test

import (
	"testing"

	"agentcall-desktop/internal/auth"
)

func TestGenerateVerifier(t *testing.T) {
	v1, err := auth.GenerateVerifier()
	if err != nil {
		t.Fatalf("GenerateVerifier: %v", err)
	}
	v2, _ := auth.GenerateVerifier()
	if v1 == v2 {
		t.Error("expected different verifiers each call")
	}
	if len(v1) < 40 {
		t.Errorf("verifier too short: %q", v1)
	}
}

func TestComputeChallenge(t *testing.T) {
	// RFC 7636 Appendix B test vector
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	got := auth.ComputeChallenge(verifier)
	if got != want {
		t.Errorf("ComputeChallenge: got %q want %q", got, want)
	}
}
