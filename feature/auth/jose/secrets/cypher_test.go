package secrets

import (
	"crypto/rand"
	"github.com/9edang/inkgate/context"
	"testing"
)

func TestNew(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := New(ctx, "base64key://")
	if err != nil {
		t.Error(err)
		return
	}

	plainKey := make([]byte, 32)
	rand.Read(plainKey)

	cypherKey, err := c.EncryptKey(ctx, plainKey)
	if err != nil {
		t.Error(err)
		return
	}

	plainText := "asdfghjklñqwertyuiozxcvbnm,"

	cypherText, err := c.Encrypt(ctx, []byte(plainText), cypherKey)
	if err != nil {
		t.Error(err)
		return
	}

	result, err := c.Decrypt(ctx, cypherText, cypherKey)
	if err != nil {
		t.Error(err)
		return
	}

	if r := string(result); r != plainText {
		t.Errorf("unexpected result: %s", r)
	}
}
