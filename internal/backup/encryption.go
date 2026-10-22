package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"golang.org/x/crypto/scrypt"
	"io"
)

const encryptedMagic = "LLBENC01"
const chunkSize = 1 << 20
const encryptedLimit = 512 << 20

func encryptionKey(password string, salt []byte) (cipher.AEAD, error) {
	if len(password) < 12 || len(password) > 256 {
		return nil, errors.New("backup password must contain 12–256 bytes")
	}
	key, e := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if e != nil {
		return nil, e
	}
	block, e := aes.NewCipher(key)
	for i := range key {
		key[i] = 0
	}
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(block)
}

// Each chunk authenticates the complete fixed header and its sequence number.
// The authenticated length prevents prefix truncation; no plaintext is published
// to the backup directory until decryption and SQLite verification both succeed.
func Encrypt(dst io.Writer, src io.Reader, size int64, password string) error {
	if size <= 0 || size > encryptedLimit {
		return errors.New("encrypted backup size must be 1 byte to 512 MB")
	}
	header := make([]byte, 40)
	copy(header, encryptedMagic)
	if _, e := rand.Read(header[8:32]); e != nil {
		return e
	}
	binary.BigEndian.PutUint64(header[32:], uint64(size))
	a, e := encryptionKey(password, header[8:24])
	if e != nil {
		return e
	}
	if _, e = dst.Write(header); e != nil {
		return e
	}
	nonce := make([]byte, 12)
	copy(nonce, header[24:32])
	buffer := make([]byte, chunkSize)
	var seq uint32
	for remaining := size; remaining > 0; {
		n := int64(chunkSize)
		if remaining < n {
			n = remaining
		}
		if _, e = io.ReadFull(src, buffer[:n]); e != nil {
			return e
		}
		binary.BigEndian.PutUint32(nonce[8:], seq)
		aad := append(append([]byte{}, header...), nonce[8:]...)
		sealed := a.Seal(nil, nonce, buffer[:n], aad)
		if _, e = dst.Write(sealed); e != nil {
			return e
		}
		seq++
		remaining -= n
	}
	return nil
}
func Decrypt(dst io.Writer, src io.Reader, password string) error {
	header := make([]byte, 40)
	if _, e := io.ReadFull(src, header); e != nil {
		return errors.New("invalid encrypted backup")
	}
	if string(header[:8]) != encryptedMagic {
		return errors.New("unsupported encrypted backup format")
	}
	size := binary.BigEndian.Uint64(header[32:])
	if size == 0 || size > encryptedLimit {
		return errors.New("invalid encrypted backup length")
	}
	a, e := encryptionKey(password, header[8:24])
	if e != nil {
		return e
	}
	nonce := make([]byte, 12)
	copy(nonce, header[24:32])
	buffer := make([]byte, chunkSize+a.Overhead())
	var seq uint32
	for remaining := size; remaining > 0; {
		n := uint64(chunkSize)
		if remaining < n {
			n = remaining
		}
		if _, e = io.ReadFull(src, buffer[:int(n)+a.Overhead()]); e != nil {
			return errors.New("encrypted backup is truncated")
		}
		binary.BigEndian.PutUint32(nonce[8:], seq)
		aad := append(append([]byte{}, header...), nonce[8:]...)
		plain, e := a.Open(nil, nonce, buffer[:int(n)+a.Overhead()], aad)
		if e != nil {
			return errors.New("incorrect password or damaged encrypted backup")
		}
		if _, e = dst.Write(plain); e != nil {
			return e
		}
		seq++
		remaining -= n
	}
	var extra [1]byte
	n, e := src.Read(extra[:])
	if n != 0 || e != io.EOF {
		return errors.New("unexpected bytes after encrypted backup")
	}
	return nil
}
