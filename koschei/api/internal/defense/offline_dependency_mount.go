package defense

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const offlineDependencySandboxCargoConfigPath = "/tmp/koschei-scratch/cargo-home/config.toml"

// AppendOfflineDependencySandboxArgs adds only read-only dependency mounts to a
// Bubblewrap argv. The caller must place these mounts after the bounded scratch
// bind and before the fixed Cargo command.
func AppendOfflineDependencySandboxArgs(args []string, authorization OfflineDependencyRuntimeAuthorization) ([]string, error) {
	if err := validateOfflineDependencyRuntimeAuthorization(authorization); err != nil {
		return nil, err
	}
	out := append([]string(nil), args...)
	out = append(out,
		"--dir", OfflineDependencyRootPath,
		"--dir", OfflineDependencyVendorPath,
		"--ro-bind", authorization.VendorPath, OfflineDependencyVendorPath,
		"--ro-bind", authorization.InventoryPath, OfflineDependencyInventoryPath,
		"--ro-bind", authorization.CargoConfigPath, OfflineDependencyCargoConfigPath,
		"--ro-bind", authorization.CargoConfigPath, offlineDependencySandboxCargoConfigPath,
	)
	return out, nil
}

func validateOfflineDependencyRuntimeAuthorization(authorization OfflineDependencyRuntimeAuthorization) error {
	if strings.TrimSpace(authorization.InventoryRef) == "" || !validOfflineDependencyDigest(authorization.InventoryHash) ||
		!validOfflineDependencyDigest(authorization.VendorTreeHash) || !validOfflineDependencyDigest(authorization.CargoManifestHash) ||
		!validOfflineDependencyDigest(authorization.CargoLockHash) || !validOfflineDependencyDigest(authorization.CargoConfigHash) {
		return errors.New(DependencyInventoryMalformed + ": runtime authorization evidence is incomplete")
	}
	if authorization.PackageName != OfflineDependencyPackageName || authorization.PackageVersion != OfflineDependencyPackageVersion ||
		authorization.FileCount <= 0 || authorization.TotalBytes < 0 || strings.TrimSpace(authorization.WorkerID) == "" ||
		normalizeDefenseSHA256Digest(authorization.WorkerImageDigest) == "" {
		return errors.New(DependencyInventoryMalformed + ": runtime authorization identity is invalid")
	}
	if authorization.NetworkAccess || authorization.DependencyResolution || authorization.VerdictAuthority {
		return errors.New(DependencyInventoryMalformed + ": runtime authorization crossed a Phase 12C boundary")
	}
	for _, item := range []struct {
		path string
		dir  bool
	}{
		{path: authorization.InventoryPath},
		{path: authorization.VendorPath, dir: true},
		{path: authorization.CargoConfigPath},
	} {
		clean := filepath.Clean(strings.TrimSpace(item.path))
		if clean == "" || clean == "." || !filepath.IsAbs(clean) || clean != item.path {
			return errors.New(DependencyInventoryPathEscape + ": runtime mount path is invalid")
		}
		info, err := os.Lstat(clean)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || (item.dir && !info.IsDir()) || (!item.dir && !info.Mode().IsRegular()) {
			return errors.New(DependencyInventoryPathEscape + ": runtime mount source is not a real file or directory")
		}
	}
	return nil
}
