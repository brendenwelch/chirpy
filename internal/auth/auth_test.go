package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateJWT(t *testing.T) {
	userID := uuid.New()
	validTok, _ := MakeJWT(userID, "secret", time.Hour)

	t.Run("Valid token", func(t *testing.T) {
		tID, err := ValidateJWT(validTok, "secret")
		if err != nil {
			t.Errorf("ValidateJWT() error = %v", err)
			return
		}
		if tID != userID {
			t.Errorf("ValidateJWT() wrong userID = %v", tID)
			return
		}
	})

	t.Run("Invalid token", func(t *testing.T) {
		_, err := ValidateJWT("invalid-token", "secret")
		if err == nil {
			t.Error("ValidateJWT() expected error")
			return
		}
	})

	t.Run("Wrong secret", func(t *testing.T) {
		_, err := ValidateJWT(validTok, "wrong")
		if err == nil {
			t.Error("ValidateJWT() expected error")
			return
		}
	})
}
