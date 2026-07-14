package blockchain

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	walletVersion       = 1
	walletCipher        = "AES-256-GCM"
	walletKDF           = "SHA256-ITERATED"
	walletKDFIterations = 120000
	walletSaltSize      = 16
	walletNonceSize     = 12
	addressPrefix       = "tc1"
)

// Wallet keeps an Ed25519 key pair in memory. PrivateKey is intentionally not
// marshalled directly; wallet files use encryptedWalletFile instead.
type Wallet struct {
	Address    string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

type encryptedWalletFile struct {
	Version       int    `json:"version"`
	Address       string `json:"address"`
	PublicKey     string `json:"public_key"`
	Cipher        string `json:"cipher"`
	KDF           string `json:"kdf"`
	KDFIterations int    `json:"kdf_iterations"`
	Salt          string `json:"salt"`
	Nonce         string `json:"nonce"`
	Ciphertext    string `json:"ciphertext"`
}

type walletSecret struct {
	PrivateKey string `json:"private_key"`
}

// WalletMetadata can be read without decrypting the private key.
type WalletMetadata struct {
	Address   string
	PublicKey string
}

// NewWallet creates a new Ed25519 wallet and derives its address from the public key.
func NewWallet() (Wallet, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Wallet{}, fmt.Errorf("generate wallet key: %w", err)
	}
	return Wallet{Address: AddressFromPublicKey(publicKey), PublicKey: publicKey, PrivateKey: privateKey}, nil
}

// AddressFromPublicKey derives a short account address from a public key.
func AddressFromPublicKey(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return addressPrefix + hex.EncodeToString(sum[:20])
}

// PublicKeyFromHex decodes a hex Ed25519 public key.
func PublicKeyFromHex(value string) (ed25519.PublicKey, error) {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: public key length %d", ErrInvalidWallet, len(decoded))
	}
	return ed25519.PublicKey(decoded), nil
}

func requirePassphrase(passphrase string) error {
	if strings.TrimSpace(passphrase) == "" {
		return fmt.Errorf("%w: passphrase is required", ErrInvalidWallet)
	}
	return nil
}

func randomBytes(size int) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("read random bytes: %w", err)
	}
	return buf, nil
}

// deriveWalletKey is a standard-library-only educational KDF. For production
// wallets, prefer a memory-hard KDF such as Argon2id or scrypt.
func deriveWalletKey(passphrase string, salt []byte, iterations int) []byte {
	seed := append([]byte(passphrase), salt...)
	sum := sha256.Sum256(seed)
	key := sum[:]
	for i := 1; i < iterations; i++ {
		nextInput := append(key, salt...)
		next := sha256.Sum256(nextInput)
		key = next[:]
	}
	out := make([]byte, 32)
	copy(out, key)
	return out
}

// SaveEncryptedWallet writes an encrypted wallet JSON file.
func SaveEncryptedWallet(path string, wallet Wallet, passphrase string) error {
	if err := requirePassphrase(passphrase); err != nil {
		return err
	}
	if wallet.Address != AddressFromPublicKey(wallet.PublicKey) {
		return fmt.Errorf("%w: address does not match public key", ErrInvalidWallet)
	}
	if len(wallet.PrivateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("%w: private key length %d", ErrInvalidWallet, len(wallet.PrivateKey))
	}

	salt, err := randomBytes(walletSaltSize)
	if err != nil {
		return err
	}
	nonce, err := randomBytes(walletNonceSize)
	if err != nil {
		return err
	}
	key := deriveWalletKey(passphrase, salt, walletKDFIterations)

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM cipher: %w", err)
	}

	secretPayload, err := json.Marshal(walletSecret{PrivateKey: hex.EncodeToString(wallet.PrivateKey)})
	if err != nil {
		return fmt.Errorf("encode wallet secret: %w", err)
	}

	file := encryptedWalletFile{
		Version:       walletVersion,
		Address:       wallet.Address,
		PublicKey:     hex.EncodeToString(wallet.PublicKey),
		Cipher:        walletCipher,
		KDF:           walletKDF,
		KDFIterations: walletKDFIterations,
		Salt:          hex.EncodeToString(salt),
		Nonce:         hex.EncodeToString(nonce),
		Ciphertext:    hex.EncodeToString(aead.Seal(nil, nonce, secretPayload, []byte(wallet.Address))),
	}

	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode wallet file: %w", err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write wallet file %q: %w", path, err)
	}
	return nil
}

func readEncryptedWalletFile(path string) (encryptedWalletFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return encryptedWalletFile{}, fmt.Errorf("read wallet file %q: %w", path, err)
	}
	var file encryptedWalletFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return encryptedWalletFile{}, fmt.Errorf("decode wallet file %q: %w", path, err)
	}
	if file.Version != walletVersion || file.Cipher != walletCipher || file.KDF != walletKDF || file.KDFIterations <= 0 {
		return encryptedWalletFile{}, fmt.Errorf("%w: unsupported wallet file parameters", ErrInvalidWallet)
	}
	return file, nil
}

// ReadWalletMetadata reads the public wallet fields without decrypting the private key.
func ReadWalletMetadata(path string) (WalletMetadata, error) {
	file, err := readEncryptedWalletFile(path)
	if err != nil {
		return WalletMetadata{}, err
	}
	publicKey, err := PublicKeyFromHex(file.PublicKey)
	if err != nil {
		return WalletMetadata{}, err
	}
	if expected := AddressFromPublicKey(publicKey); file.Address != expected {
		return WalletMetadata{}, fmt.Errorf("%w: address does not match public key", ErrInvalidWallet)
	}
	return WalletMetadata{Address: file.Address, PublicKey: file.PublicKey}, nil
}

// LoadEncryptedWallet decrypts an encrypted wallet file using a passphrase.
func LoadEncryptedWallet(path string, passphrase string) (Wallet, error) {
	if err := requirePassphrase(passphrase); err != nil {
		return Wallet{}, err
	}
	file, err := readEncryptedWalletFile(path)
	if err != nil {
		return Wallet{}, err
	}

	publicKey, err := PublicKeyFromHex(file.PublicKey)
	if err != nil {
		return Wallet{}, err
	}
	if expected := AddressFromPublicKey(publicKey); file.Address != expected {
		return Wallet{}, fmt.Errorf("%w: address does not match public key", ErrInvalidWallet)
	}

	salt, err := hex.DecodeString(file.Salt)
	if err != nil {
		return Wallet{}, fmt.Errorf("decode wallet salt: %w", err)
	}
	nonce, err := hex.DecodeString(file.Nonce)
	if err != nil {
		return Wallet{}, fmt.Errorf("decode wallet nonce: %w", err)
	}
	ciphertext, err := hex.DecodeString(file.Ciphertext)
	if err != nil {
		return Wallet{}, fmt.Errorf("decode wallet ciphertext: %w", err)
	}

	key := deriveWalletKey(passphrase, salt, file.KDFIterations)
	block, err := aes.NewCipher(key)
	if err != nil {
		return Wallet{}, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Wallet{}, fmt.Errorf("create GCM cipher: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, []byte(file.Address))
	if err != nil {
		return Wallet{}, fmt.Errorf("%w: decrypt private key: %w", ErrInvalidWallet, err)
	}

	var secret walletSecret
	if err := json.Unmarshal(plaintext, &secret); err != nil {
		return Wallet{}, fmt.Errorf("decode wallet secret: %w", err)
	}
	privateKeyBytes, err := hex.DecodeString(secret.PrivateKey)
	if err != nil {
		return Wallet{}, fmt.Errorf("decode private key: %w", err)
	}
	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return Wallet{}, fmt.Errorf("%w: private key length %d", ErrInvalidWallet, len(privateKeyBytes))
	}
	privateKey := ed25519.PrivateKey(privateKeyBytes)
	if !privateKey.Public().(ed25519.PublicKey).Equal(publicKey) {
		return Wallet{}, errors.New("wallet private key does not match public key")
	}

	return Wallet{Address: file.Address, PublicKey: publicKey, PrivateKey: privateKey}, nil
}
