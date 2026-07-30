package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/DevAnimecx/deltacode/pkg/models"
)

const configDir = ".delta"
const configFile = "config.json"

var machineKey []byte

func init() {
	machineKey = deriveMachineKey()
}

func deriveMachineKey() []byte {
	hostname, _ := os.Hostname()
	key := []byte("d3lt4-c0d3-k3y-" + hostname)
	hashed := make([]byte, 32)
	for i := 0; i < 32; i++ {
		hashed[i] = key[i%len(key)] ^ byte(i*17)
	}
	return hashed
}

type Manager struct {
	configPath string
	config     models.Config
}

func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home dir: %w", err)
	}
	dir := filepath.Join(home, configDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("cannot create config dir: %w", err)
	}
	m := &Manager{
		configPath: filepath.Join(dir, configFile),
		config: models.Config{
			DefaultProvider: "openai",
			DefaultModel:    "gpt-4o",
			Memory: models.MemoryConfig{
				Enabled: true,
			},
		},
	}
	if err := m.load(); err != nil {
		m.Save()
	}
	return m, nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}
	var cfg models.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	for i := range cfg.Providers {
		decrypted, err := decrypt(cfg.Providers[i].APIKey)
		if err == nil {
			cfg.Providers[i].APIKey = decrypted
		}
	}
	m.config = cfg
	return nil
}

func (m *Manager) Save() error {
	cfg := m.config
	for i := range cfg.Providers {
		encrypted, err := encrypt(cfg.Providers[i].APIKey)
		if err != nil {
			return err
		}
		cfg.Providers[i].APIKey = encrypted
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.configPath, data, 0600)
}

func (m *Manager) GetConfig() models.Config {
	return m.config
}

func (m *Manager) AddProvider(p models.ProviderConfig) error {
	for i, existing := range m.config.Providers {
		if existing.Name == p.Name {
			m.config.Providers[i] = p
			return m.Save()
		}
	}
	m.config.Providers = append(m.config.Providers, p)
	return m.Save()
}

func (m *Manager) RemoveProvider(name string) error {
	idx := -1
	for i, p := range m.config.Providers {
		if p.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("provider %q not found", name)
	}
	m.config.Providers = append(m.config.Providers[:idx], m.config.Providers[idx+1:]...)
	return m.Save()
}

func (m *Manager) GetProvider(name string) (*models.ProviderConfig, error) {
	for _, p := range m.config.Providers {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("provider %q not found", name)
}

func (m *Manager) ListProviders() []models.ProviderConfig {
	return m.config.Providers
}

func (m *Manager) SetDefault(name string) error {
	for _, p := range m.config.Providers {
		if p.Name == name {
			m.config.DefaultProvider = name
			if len(p.Models) > 0 {
				m.config.DefaultModel = p.Models[0]
			}
			return m.Save()
		}
	}
	return fmt.Errorf("provider %q not found", name)
}

func encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(machineKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(machineKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
