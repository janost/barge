package exec

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"

	"archive/tar"
)

const (
	markerStart = "BRIDGE_START"
	markerEnd   = "BRIDGE_END"
	markerOK    = "BRIDGE_OK"
	// Max encoded size for uploads (256KB). ECS execute-command passes the
	// command string through SSM which has practical payload limits.
	maxUploadSize = 256 * 1024
)

// CopyFromContainer downloads a file or directory from a remote ECS container.
func CopyFromContainer(cluster, taskID, container, remotePath, localPath string) error {
	dir := filepath.Dir(remotePath)
	base := filepath.Base(remotePath)

	command := fmt.Sprintf(
		"/bin/sh -c 'echo %s; tar czf - -C %s %s | base64; echo %s'",
		markerStart, shellQuote(dir), shellQuote(base), markerEnd,
	)

	out, err := runInContainer(cluster, taskID, container, command)
	if err != nil {
		return fmt.Errorf("remote command failed: %w", err)
	}

	payload, err := extractPayload(out, markerStart, markerEnd)
	if err != nil {
		return fmt.Errorf("failed to extract data (container may lack tar/base64, or path does not exist): %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(payload)))
	if err != nil {
		return fmt.Errorf("base64 decode: %w", err)
	}

	return extractTarGz(decoded, localPath)
}

// CopyToContainer uploads a local file or directory to a remote ECS container.
func CopyToContainer(cluster, taskID, container, localPath, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("local path: %w", err)
	}

	encoded, err := createTarGzBase64(localPath, info)
	if err != nil {
		return fmt.Errorf("archive local files: %w", err)
	}

	if len(encoded) > maxUploadSize {
		return fmt.Errorf("encoded payload is %d bytes (limit %d bytes / 256KB); file too large for ECS exec transfer",
			len(encoded), maxUploadSize)
	}

	command := fmt.Sprintf(
		"/bin/sh -c 'echo %s | base64 -d | tar xzf - -C %s; echo %s'",
		encoded, shellQuote(remotePath), markerOK,
	)

	out, err := runInContainer(cluster, taskID, container, command)
	if err != nil {
		return fmt.Errorf("remote command failed: %w", err)
	}

	if !bytes.Contains(out, []byte(markerOK)) {
		return fmt.Errorf("upload may have failed: confirmation marker not found in output")
	}

	return nil
}

// runInContainer executes a command inside an ECS container via aws ecs execute-command
// and returns the captured stdout. Unlike exec.Exec, this runs as a subprocess
// (not syscall.Exec) so we can capture output.
func runInContainer(cluster, taskID, container, command string) ([]byte, error) {
	awsBin, err := osexec.LookPath("aws")
	if err != nil {
		return nil, fmt.Errorf("aws CLI not found: %w", err)
	}

	cmd := osexec.Command(awsBin, "ecs", "execute-command",
		"--cluster", cluster,
		"--task", taskID,
		"--container", container,
		"--interactive",
		"--command", command,
	)
	cmd.Stdin = os.Stdin
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return stdout.Bytes(), nil
}

// extractPayload finds the content between start and end markers in output.
func extractPayload(data []byte, start, end string) ([]byte, error) {
	startMarker := []byte(start + "\n")
	endMarker := []byte("\n" + end)

	startIdx := bytes.Index(data, startMarker)
	if startIdx == -1 {
		return nil, fmt.Errorf("start marker %q not found", start)
	}
	startIdx += len(startMarker)

	endIdx := bytes.Index(data[startIdx:], endMarker)
	if endIdx == -1 {
		return nil, fmt.Errorf("end marker %q not found (command may have failed)", end)
	}

	return data[startIdx : startIdx+endIdx], nil
}

// extractTarGz decompresses and untars gzipped data into dst.
func extractTarGz(data []byte, dst string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		target := filepath.Join(dst, hdr.Name)

		// Prevent path traversal
		if !filepath.IsAbs(target) {
			target = filepath.Clean(target)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

// createTarGzBase64 archives localPath into a tar.gz and returns the base64-encoded string.
func createTarGzBase64(localPath string, info os.FileInfo) (string, error) {
	var buf bytes.Buffer
	b64w := base64.NewEncoder(base64.StdEncoding, &buf)
	gzw := gzip.NewWriter(b64w)
	tw := tar.NewWriter(gzw)

	if info.IsDir() {
		err := filepath.Walk(localPath, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(localPath, path)
			if err != nil {
				return err
			}
			hdr, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				return err
			}
			hdr.Name = rel
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if !fi.IsDir() {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()
				if _, err := io.Copy(tw, f); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	} else {
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return "", err
		}
		hdr.Name = info.Name()
		if err := tw.WriteHeader(hdr); err != nil {
			return "", err
		}
		f, err := os.Open(localPath)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := io.Copy(tw, f); err != nil {
			return "", err
		}
	}

	// Close in order: tar -> gzip -> base64
	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gzw.Close(); err != nil {
		return "", err
	}
	if err := b64w.Close(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// shellQuote wraps a string in single quotes for /bin/sh, escaping embedded quotes.
func shellQuote(s string) string {
	// Replace ' with '\'' (end quote, escaped quote, start quote)
	escaped := bytes.ReplaceAll([]byte(s), []byte("'"), []byte("'\\''"))
	return "'" + string(escaped) + "'"
}
