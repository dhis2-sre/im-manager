package instance

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

func commandExecutor(cmd *exec.Cmd, cluster model.Cluster) (stdout []byte, stderr []byte, err error) {
	// Give the helm invocation its own repository config file. Concurrent helmfile syncs otherwise
	// race on the shared repositories.yaml: one process's repo add is clobbered by another's write
	// between the add and the upgrade, failing with "repo not found". The chart cache stays shared
	// so indexes and charts are not re-downloaded per deploy.
	repoConfigDir, err := os.MkdirTemp("", "helm-repo-config")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create helm repository config dir: %v", err)
	}
	defer func() {
		if errR := os.RemoveAll(repoConfigDir); errR != nil && err == nil {
			err = fmt.Errorf("failed to clean up helm repository config dir: %v", errR)
		}
	}()
	cmd.Env = append(cmd.Env, fmt.Sprintf("HELM_REPOSITORY_CONFIG=%s", filepath.Join(repoConfigDir, "repositories.yaml")))

	if cluster.Configuration == nil {
		return runCommand(cmd)
	}

	kubeCfg, err := kube.DecryptYaml(cluster.Configuration)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt kubernetes config: %v", err)
	}

	file, err := os.CreateTemp("", "kubectl")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		// remove the file even if closing it fails. os.Remove is actually making syscall unlink
		// unlink deletes a name and the file if the name was the last link to the file.
		// If we fail to close the file it will remain in existence until the last file descriptor
		// referring to it is closed. As we don't return the file, this should be done once a GC
		// occurs.

		errC := file.Close()
		errR := os.Remove(file.Name())
		errMsg := joinErrors(err, errC, errR)
		if errMsg != "" {
			err = fmt.Errorf("error handling kube config %q: %s", file.Name(), errMsg)
		}
	}()

	_, err = file.Write(kubeCfg)
	if err != nil {
		return nil, nil, err
	}

	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", file.Name()))
	return runCommand(cmd)
}

func joinErrors(errs ...error) string {
	var errMsgs []string
	for _, err := range errs {
		if err != nil {
			errMsgs = append(errMsgs, err.Error())
		}
	}
	return strings.Join(errMsgs, ", ")
}

func runCommand(cmd *exec.Cmd) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return stdout.Bytes(), stderr.Bytes(), err
}
