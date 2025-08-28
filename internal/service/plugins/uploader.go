package plugins

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	widgetsrepo "github.com/faciam-dev/gcfm/internal/repository/widgets"
)

// UploadOptions controls how a plugin upload should be handled.
type UploadOptions struct {
	TenantScope string
	Tenants     []string
}

// Manifest represents a plugin manifest inside the uploaded archive.
type Manifest struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Type         string         `json:"type"`
	Scopes       []string       `json:"scopes"`
	Enabled      *bool          `json:"enabled,omitempty"`
	Description  *string        `json:"description,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Homepage     *string        `json:"homepage,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
}

// UploadedWidget contains information about the stored widget.
type UploadedWidget struct {
	ID           string
	Name         string
	Version      string
	Type         string
	Scopes       []string
	Enabled      bool
	Description  *string
	Capabilities []string
	Homepage     *string
	Meta         map[string]any
	TenantScope  string
	Tenants      []string
	UpdatedAt    time.Time
	PackageSize  int64
}

// WidgetsRepo defines the repository for widgets.
type WidgetsRepo interface {
	Upsert(ctx context.Context, r widgetsrepo.Row) error
}

// WidgetsNotifier notifies other nodes of widget changes.
type WidgetsNotifier interface {
	NotifyWidgetChanged(ctx context.Context, id string) error
	NotifyWidgetRemoved(ctx context.Context, id string) error
}

// Logger represents the minimal logging interface used by the uploader.
type Logger interface {
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Uploader handles plugin uploads.
type Uploader struct {
	Repo      WidgetsRepo
	Notifier  WidgetsNotifier
	Logger    Logger
	AcceptExt []string
	TmpDir    string
	StoreDir  string
}

// clientError represents errors that should be returned to the client.
type clientError struct{ err error }

func (e clientError) Error() string { return e.err.Error() }
func (e clientError) Unwrap() error { return e.err }

// IsClientErr reports whether an error is a client error.
func IsClientErr(err error) bool {
	var ce clientError
	return errors.As(err, &ce)
}

// HandleUpload processes the uploaded plugin file and stores metadata.
func (u *Uploader) HandleUpload(ctx context.Context, f multipart.File, filename string, opt UploadOptions) (*UploadedWidget, error) {
	if !u.accept(filename) {
		return nil, clientError{fmt.Errorf("unsupported file extension: %s", filename)}
	}

	tmpPath, size, err := u.saveTemp(f)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpPath)

	man, err := ExtractManifest(tmpPath)
	if err != nil {
		return nil, clientError{fmt.Errorf("invalid package: %w", err)}
	}
	if err := validateManifest(man); err != nil {
		return nil, clientError{fmt.Errorf("invalid manifest: %w", err)}
	}

	enabled := true
	if man.Enabled != nil {
		enabled = *man.Enabled
	}

	tenantScope := strings.TrimSpace(opt.TenantScope)
	if tenantScope == "" {
		tenantScope = "system"
		for _, s := range man.Scopes {
			if s == "tenant" {
				tenantScope = "tenant"
				break
			}
		}
	} else if tenantScope != "system" && tenantScope != "tenant" {
		return nil, clientError{fmt.Errorf("invalid tenant_scope: %s", tenantScope)}
	}

	w := &UploadedWidget{
		ID:           man.ID,
		Name:         man.Name,
		Version:      man.Version,
		Type:         man.Type,
		Scopes:       man.Scopes,
		Enabled:      enabled,
		Description:  man.Description,
		Capabilities: man.Capabilities,
		Homepage:     man.Homepage,
		Meta:         man.Meta,
		TenantScope:  tenantScope,
		Tenants:      opt.Tenants,
		UpdatedAt:    time.Now().UTC(),
		PackageSize:  size,
	}

	if u.Repo == nil {
		if u.Logger != nil {
			u.Logger.Error("widget repo not configured")
		}
		return nil, errors.New("widget repo not configured")
	}
	if err := u.Repo.Upsert(ctx, ToRow(w)); err != nil {
		if u.Logger != nil {
			u.Logger.Error("widget upsert failed", "id", w.ID, "version", w.Version, "tenant_scope", w.TenantScope, "err", err)
		}
		return nil, err
	}

	if u.Notifier != nil {
		if err := u.Notifier.NotifyWidgetChanged(ctx, w.ID); err != nil && u.Logger != nil {
			u.Logger.Warn("notify failed", "id", w.ID, "err", err)
		}
	}

	if u.StoreDir != "" {
		if err := u.persist(tmpPath, filename, w.ID); err != nil && u.Logger != nil {
			u.Logger.Error("widget file persistence failed", "id", w.ID, "filename", filename, "err", err)
		}
	}
	return w, nil
}

func (u *Uploader) persist(src, orig, id string) error {
	if u.StoreDir == "" {
		return nil
	}
	if err := os.MkdirAll(u.StoreDir, 0o750); err != nil {
		return err
	}
	src = filepath.Clean(src)
	dst := filepath.Clean(filepath.Join(u.StoreDir, fmt.Sprintf("%s_%s", id, filepath.Base(orig))))
	in, err := os.Open(src) // #nosec G304 -- src is a temp file under our control
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func (u *Uploader) accept(fn string) bool {
	lower := strings.ToLower(fn)
	for _, ext := range u.AcceptExt {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func (u *Uploader) saveTemp(f multipart.File) (string, int64, error) {
	dir := u.TmpDir
	if dir == "" {
		dir = os.TempDir()
	}
	tmp, err := os.CreateTemp(dir, "plugin*")
	if err != nil {
		return "", 0, err
	}
	defer tmp.Close()
	n, err := io.Copy(tmp, f)
	if err != nil {
		return "", 0, err
	}
	return tmp.Name(), n, nil
}

// ExtractManifest extracts manifest.json or plugin.json from the archive at path.
func ExtractManifest(path string) (*Manifest, error) {
	p := filepath.Clean(path)
	f, err := os.Open(p) // #nosec G304 -- path cleaned before use
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, 4)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, err
	}

	switch {
	// ZIP files start with "PK\x03\x04"
	case header[0] == 0x50 && header[1] == 0x4b && header[2] == 0x03 && header[3] == 0x04:
		return extractManifestFromZip(p)
	// Gzip-compressed tarballs start with 0x1f,0x8b
	case header[0] == 0x1f && header[1] == 0x8b:
		return extractManifestFromTgz(p)
	default:
		return nil, fmt.Errorf("unsupported archive")
	}
}

func extractManifestFromZip(path string) (*Manifest, error) {
	p := filepath.Clean(path)
	zr, err := zip.OpenReader(p) // #nosec G304 -- path cleaned and validated
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		if zipslip(name) {
			return nil, errors.New("zip slip detected")
		}
		base := filepath.Base(name)
		if base == "manifest.json" || base == "plugin.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			return decodeManifest(b)
		}
	}
	return nil, errors.New("manifest.json not found")
}

func extractManifestFromTgz(path string) (*Manifest, error) {
	p := filepath.Clean(path)
	f, err := os.Open(p) // #nosec G304 -- path cleaned before use
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := filepath.ToSlash(hdr.Name)
		if zipslip(name) {
			return nil, errors.New("tar slip detected")
		}
		base := filepath.Base(name)
		if base == "manifest.json" || base == "plugin.json" {
			b, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			return decodeManifest(b)
		}
	}
	return nil, errors.New("manifest.json not found")
}

func decodeManifest(b []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func zipslip(name string) bool {
	cleaned := filepath.Clean(name)
	return filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "../")
}

func validateManifest(m *Manifest) error {
	if m.ID == "" || m.Name == "" || m.Version == "" || m.Type == "" {
		return fmt.Errorf("id/name/version/type required")
	}
	if m.Type != "widget" {
		return fmt.Errorf("unsupported type: %s", m.Type)
	}
	if len(m.Scopes) == 0 {
		m.Scopes = []string{"system"}
	}
	return nil
}

// ToRow converts UploadedWidget to repository Row.
func ToRow(w *UploadedWidget) widgetsrepo.Row {
	tenants := w.Tenants
	if w.TenantScope == "tenant" && tenants == nil {
		tenants = []string{}
	}
	return widgetsrepo.Row{
		ID:           w.ID,
		Name:         w.Name,
		Version:      w.Version,
		Type:         w.Type,
		Scopes:       w.Scopes,
		Enabled:      w.Enabled,
		Description:  w.Description,
		Capabilities: w.Capabilities,
		Homepage:     w.Homepage,
		Meta:         w.Meta,
		TenantScope:  w.TenantScope,
		Tenants:      tenants,
	}
}
