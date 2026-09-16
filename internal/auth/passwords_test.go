package auth

import "testing"

func TestPasswordHashAndVerification(t *testing.T) {
	hash, err := HashPassword("SecurePassword123!")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "SecurePassword123!" {
		t.Fatal("password was not hashed")
	}
	if !VerifyPassword("SecurePassword123!", hash) {
		t.Fatal("correct password was rejected")
	}
	if VerifyPassword("WrongPassword321!", hash) {
		t.Fatal("incorrect password was accepted")
	}
}
