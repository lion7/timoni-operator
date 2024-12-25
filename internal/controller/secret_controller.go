/*
Copyright 2024 Gerard de Leeuw.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SecretReconciler reconciles a Secret object
type SecretReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=secrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=secrets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if req.Namespace != "kube-system" {
		return ctrl.Result{}, nil
	}

	secret := corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, &secret); err != nil {
		if errors.IsNotFound(err) {
			// TODO: delete bundle?
			return ctrl.Result{}, nil
		}
		logger.Error(err, fmt.Sprintf("cannot reconcile Secret %s", req.NamespacedName))
		return ctrl.Result{}, err
	}

	if string(secret.Type) != "timoni.sh/bundle" {
		return ctrl.Result{}, nil
	}

	tmpDir, err := os.MkdirTemp("", "timoni-operator")
	if err != nil {
		return ctrl.Result{}, nil
	}
	//goland:noinspection GoUnhandledErrorResult
	defer os.RemoveAll(tmpDir)

	files, err := writeSecretDataToTempDir(tmpDir, secret.Data)
	if err != nil {
		return ctrl.Result{}, err
	}

	args := []string{"bundle", "apply"}
	for _, file := range files {
		args = append(args, "-f", file)
	}
	cmd := exec.Command("timoni", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}

func writeSecretDataToTempDir(tmpDir string, data map[string][]byte) ([]string, error) {
	var files []string
	for name, bytes := range data {
		if !strings.HasSuffix(name, ".cue") {
			continue
		}
		file := path.Join(tmpDir, name)
		if err := os.WriteFile(file, bytes, 0644); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}
