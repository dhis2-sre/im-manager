package instance

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func NewPodBackupSource(logger *slog.Logger, clientset *kubernetes.Clientset, config *rest.Config, namespace, podName, containerName, sourcePath string) *PodBackupSource {
	return &PodBackupSource{logger, clientset, config, namespace, podName, containerName, sourcePath}
}

// PodBackupSource implements BackupSource for Kubernetes pods
type PodBackupSource struct {
	logger        *slog.Logger
	clientset     *kubernetes.Clientset
	config        *rest.Config
	namespace     string
	podName       string
	containerName string
	sourcePath    string
}

// List implements BackupSource interface
func (p *PodBackupSource) List(ctx context.Context) (<-chan BackupObject, error) {
	if err := p.checkTarExists(ctx); err != nil {
		return nil, err
	}

	ch := make(chan BackupObject)

	srcPath := filepath.Clean(p.sourcePath)
	if srcPath == "." || srcPath == "/" {
		return nil, fmt.Errorf("invalid source path: %s", srcPath)
	}

	tarCmd := []string{
		"tar",
		"--warning=no-timestamp",
		"--create",
		"--file=-",
		"--directory", srcPath,
		".",
	}

	p.logger.InfoContext(ctx, "list tar cmd", "cmd", tarCmd)

	reader, err := p.execInPod(ctx, tarCmd)
	if err != nil {
		return nil, err
	}

	// Start a goroutine to read the tar stream
	go func() {
		defer close(ch)
		tr := tar.NewReader(reader)

		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				select {
				case ch <- BackupObject{Err: fmt.Errorf("read tar: %v", err)}:
				case <-ctx.Done():
				}
				return
			}

			// Skip directories from the tar stream, not the files inside
			if header.Typeflag == tar.TypeDir {
				continue
			}

			path := filepath.Clean(header.Name)
			if strings.HasPrefix(path, "../") {
				continue // Skip files outside the target directory
			}

			p.logger.InfoContext(ctx, "File in tar", "path", path, "size", header.Size, "modTime", header.ModTime)

			select {
			case ch <- BackupObject{
				Path:         path,
				Size:         header.Size,
				LastModified: header.ModTime,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// checkTarExists verifies the tar command is available in the container
func (p *PodBackupSource) checkTarExists(ctx context.Context) error {
	cmd := []string{"tar", "--version"}
	req := p.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(p.podName).
		Namespace(p.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: p.containerName,
			Command:   cmd,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(p.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		return fmt.Errorf("tar not available in container: %v", err)
	}

	p.logger.Debug("tar command output", "output", stdout.String())
	return nil
}

// Get implements BackupSource interface
func (p *PodBackupSource) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	path = filepath.Clean(path)
	if strings.HasPrefix(path, "../") {
		return nil, fmt.Errorf("invalid path: %s", path)
	}

	catCmd := []string{"cat", filepath.Join(p.sourcePath, path)}

	p.logger.InfoContext(ctx, "get file cmd", "cmd", catCmd)

	return p.execInPod(ctx, catCmd)
}

// execInPod executes a command in the pod and returns the output
func (p *PodBackupSource) execInPod(ctx context.Context, command []string) (io.ReadCloser, error) {
	req := p.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(p.podName).
		Namespace(p.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: p.containerName,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(p.config, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %v", err)
	}

	pr, pw := io.Pipe()

	var stderr bytes.Buffer

	// Start a goroutine to handle the exec streaming
	go func() {
		defer func(pw *io.PipeWriter) {
			err := pw.Close()
			if err != nil {
				p.logger.ErrorContext(ctx, "failed to close pipe writer", "err", err)
			}
		}(pw)

		err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdout: pw,
			Stderr: &stderr,
		})
		if err != nil {
			err := pw.CloseWithError(fmt.Errorf("exec error: %v, stderr: %s", err, stderr.String()))
			if err != nil {
				p.logger.ErrorContext(ctx, "failed to close pipe writer", "err", err)
			}
		}
	}()

	return pr, nil
}
