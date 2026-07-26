package targets

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/konveyor/tackle2-hub/shared/api"
	"github.com/konveyor/test-harness/pkg/util"
	"gopkg.in/yaml.v2"
)

// writes dependencies.yaml for Hub tests.
func (t *TackleHubTarget) writeHubDependenciesOutput(completedTask *api.Task, taskID, appID uint, outputDir string) error {
	log := util.GetLogger()
	dst := filepath.Join(outputDir, "dependencies.yaml")

	err := t.tryDepsFileAttachment(completedTask, dst)
	if err == nil {
		log.Info("Wrote dependencies.yaml from Hub file attachment", "path", dst)
		return nil
	}
	if !errors.Is(err, errDepsAttachmentNotFound) {
		return err
	}

	err = t.writeDependenciesFromTaskTarball(taskID, outputDir)
	if err == nil {
		log.Info("Wrote dependencies.yaml from Hub task attached tarball", "path", dst)
		return nil
	}
	if !errors.Is(err, errDepsAttachmentNotFound) {
		return err
	}

	hubDeps, err := t.client.Application.Select(appID).Analysis.ListDependencies()
	if err != nil {
		return fmt.Errorf("failed to get analysis dependencies: %w", err)
	}
	flat := hubTechDependenciesToDepsFlat(hubDeps)
	depYAML, err := yaml.Marshal(flat)
	if err != nil {
		return fmt.Errorf("failed to marshal dependencies: %w", err)
	}
	if err := os.WriteFile(dst, depYAML, 0644); err != nil {
		return fmt.Errorf("failed to write dependencies.yaml: %w", err)
	}
	log.Info("Wrote dependencies.yaml from Hub", "path", dst, "flatItems", len(flat))
	return nil
}

var errDepsAttachmentNotFound = errors.New("deps.yaml not found in attachments")

func (t *TackleHubTarget) tryDepsFileAttachment(completedTask *api.Task, dst string) error {
	for _, att := range completedTask.Attached {
		if !isDepsAttachmentName(att.Name) {
			continue
		}
		if err := t.client.File.Get(att.ID, dst); err != nil {
			return fmt.Errorf("download deps via Hub /files/%d (%q): %w", att.ID, att.Name, err)
		}
		return nil
	}
	return errDepsAttachmentNotFound
}

func isDepsAttachmentName(name string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(name)))
	return base == "deps.yaml" || base == "dependencies.yaml"
}

func (t *TackleHubTarget) writeDependenciesFromTaskTarball(taskID uint, outputDir string) error {
	tmpDir, err := os.MkdirTemp("", "koncur-hub-attached-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := t.client.Task.GetAttached(taskID, tmpDir); err != nil {
		return fmt.Errorf("download task attachments: %w", err)
	}

	path, err := findDepsYAMLInTree(tmpDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errDepsAttachmentNotFound
		}
		return err
	}
	dst := filepath.Join(outputDir, "dependencies.yaml")
	if err := copyFile(path, dst); err != nil {
		return fmt.Errorf("copy to dependencies.yaml: %w", err)
	}
	return nil
}

func findDepsYAMLInTree(root string) (string, error) {
	p, err := walkForDepsYAML(root)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".tar") && !strings.HasSuffix(lower, ".tar.gz") && !strings.HasSuffix(lower, ".tgz") {
			continue
		}
		sub, err := os.MkdirTemp("", "koncur-hub-tar-*")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(sub)
		if err := extractTarArchive(filepath.Join(root, name), sub); err != nil {
			return "", fmt.Errorf("extract %s: %w", name, err)
		}
		if p, err := walkForDepsYAML(sub); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

func walkForDepsYAML(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		if base == "deps.yaml" || base == "dependencies.yaml" {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", os.ErrNotExist
	}
	return found, nil
}

type gzipReadCloser struct {
	gz *gzip.Reader
	f  *os.File
}

func (g *gzipReadCloser) Read(p []byte) (n int, err error) { return g.gz.Read(p) }
func (g *gzipReadCloser) Close() error {
	err := g.gz.Close()
	err2 := g.f.Close()
	if err != nil {
		return err
	}
	return err2
}

func openTarStream(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var hdr [2]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		f.Close()
		return nil, err
	}
	if _, err := f.Seek(0, 0); err != nil {
		f.Close()
		return nil, err
	}
	lower := strings.ToLower(path)
	gzipByExt := strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz")
	gzipByMagic := hdr[0] == 0x1f && hdr[1] == 0x8b
	if gzipByExt || gzipByMagic {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &gzipReadCloser{gz: gzr, f: f}, nil
	}
	return f, nil
}

func extractTarArchive(src, destDir string) error {
	r, err := openTarStream(src)
	if err != nil {
		return err
	}
	defer r.Close()

	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, header.Name)
		rel, err := filepath.Rel(destDir, filepath.Clean(target))
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := header.Mode
			if mode == 0 {
				mode = 0644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode&0777))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
