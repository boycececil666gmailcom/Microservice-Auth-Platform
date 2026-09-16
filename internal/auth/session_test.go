package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRefreshTokenRotationIsAtomicAndOneTime(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	oldToken := "old-bearer-token"
	newToken := "new-bearer-token"
	oldKey := refreshTokenKey(oldToken)
	newKey := refreshTokenKey(newToken)
	if err := client.Set(ctx, oldKey, "alice@example.com", 0).Err(); err != nil {
		t.Fatal(err)
	}

	result, err := rotateRefreshTokenScript.Run(
		ctx, client, []string{oldKey, newKey}, "alice@example.com", 3600,
	).Text()
	if err != nil {
		t.Fatal(err)
	}
	if result != "alice@example.com" {
		t.Fatalf("rotation returned %q", result)
	}
	if client.Exists(ctx, oldKey).Val() != 0 || client.Get(ctx, newKey).Val() != "alice@example.com" {
		t.Fatal("rotation did not atomically replace the token")
	}

	_, err = rotateRefreshTokenScript.Run(
		ctx, client, []string{oldKey, refreshTokenKey("another-token")}, "alice@example.com", 3600,
	).Text()
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("reused refresh token returned %v, want redis.Nil", err)
	}
}
