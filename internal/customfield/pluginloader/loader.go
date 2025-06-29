package pluginloader

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"

	"go.uber.org/zap"

	"github.com/faciam-dev/gcfm/internal/customfield"
)

// Enabled toggles plugin loading. It is true by default.
var Enabled = true

// PublicKeyPath specifies the file containing the ed25519 public key
// used to verify plugin signatures. When empty, loading will fail.
var PublicKeyPath string

// verifySignature checks that path and path+".sig" match the public key.
func verifySignature(path string) bool {
	if PublicKeyPath == "" {
		return false
	}
	pubData, err := os.ReadFile(PublicKeyPath)
	if err != nil {
		return false
	}
	pubKeyBytes, err := hex.DecodeString(strings.TrimSpace(string(pubData)))
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return false
	}
	pub := ed25519.PublicKey(pubKeyBytes)

	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	sigData, err := os.ReadFile(path + ".sig")
	if err != nil {
		return false
	}
	sigBytes, err := hex.DecodeString(strings.TrimSpace(string(sigData)))
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pub, data, sigBytes)
}

// DefaultDir returns the path where validator plugins are stored for the
// current OS.
func DefaultDir() string {
	if runtime.GOOS == "windows" {
		dir := os.Getenv("APPDATA")
		if dir == "" {
			if h, err := os.UserHomeDir(); err == nil {
				dir = filepath.Join(h, "AppData", "Roaming")
			}
		}
		return filepath.Join(dir, "customfield", "plugins")
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".customfield", "plugins")
	}
	return "./plugins"
}

// LoadAll loads all plugins from dir.
// If dir is empty, DefaultDir() is used.
// It returns an error if loading any plugin fails.
func LoadAll(dir string, logger *zap.SugaredLogger) error {
	if !Enabled {
		logger.Infow("plugin loading disabled")
		return nil
	}
	if dir == "" {
		dir = DefaultDir()
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.so"))
	if err != nil {
		logger.Warnw("failed to read plugin directory", "dir", dir, "err", err)
	}
	for _, f := range files {
		if !verifySignature(f) {
			logger.Warnw("invalid signature", "file", f)
			continue
		}
		p, err := plugin.Open(f)
		if err != nil {
			logger.Warnw("plugin open failed", "file", f, "err", err)
			continue
		}
		sym, err := p.Lookup("New")
		if err != nil {
			logger.Warnw("symbol missing", "file", f, "err", err)
			continue
		}
		ctor, ok := sym.(func() customfield.ValidatorPlugin)
		if !ok {
			logger.Warnw("invalid type", "file", f)
			continue
		}
		inst := ctor()
		if err := customfield.RegisterValidator(inst.Name(), inst.Validate); err != nil {
			if errors.Is(err, customfield.ErrValidatorExists) {
				logger.Warnw("validator already registered", "name", inst.Name(), "file", f)
				continue
			}
			return err
		}
		logger.Infow("validator plugin loaded", "name", inst.Name(), "file", f)
	}
	return nil
}
