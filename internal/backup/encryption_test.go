package backup

import (
	"bytes"
	"testing"
)

func TestEncryptedBackupRoundTripAndTamper(t *testing.T) {
	plain := bytes.Repeat([]byte("backup fixture bytes"), 120000)
	var encrypted bytes.Buffer
	if e := Encrypt(&encrypted, bytes.NewReader(plain), int64(len(plain)), "correct long password"); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := Decrypt(&out, bytes.NewReader(encrypted.Bytes()), "correct long password"); e != nil || !bytes.Equal(out.Bytes(), plain) {
		t.Fatal("roundtrip", e)
	}
	for _, kind := range []string{"password", "tamper", "truncate", "trailing", "length"} {
		t.Run(kind, func(t *testing.T) {
			data := append([]byte{}, encrypted.Bytes()...)
			password := "correct long password"
			switch kind {
			case "password":
				password = "incorrect long password"
			case "tamper":
				data[len(data)-5] ^= 1
			case "truncate":
				data = data[:len(data)-20]
			case "trailing":
				data = append(data, 0)
			case "length":
				data[39] ^= 1
			}
			if e := Decrypt(&bytes.Buffer{}, bytes.NewReader(data), password); e == nil {
				t.Fatal("accepted altered envelope")
			}
		})
	}
}
