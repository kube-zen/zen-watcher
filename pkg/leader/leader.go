// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package leader

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// AnnotationRole is the annotation key for the role (leader/follower)
	AnnotationRole = "zen-lead/role"
	// RoleLeader indicates this pod is the leader
	RoleLeader = "leader"
	// RoleFollower indicates this pod is a follower
	RoleFollower = "follower"
	// DefaultCheckInterval is the default interval for checking leader status
	DefaultCheckInterval = 5 * time.Second
)

// Checker checks if the current pod is the leader using zen-lead annotations
type Checker struct {
	clientset    kubernetes.Interface
	podName      string
	podNamespace string
	checkInterval time.Duration
	lastRole     string
	lastCheck    time.Time
}

// NewChecker creates a new leader checker
func NewChecker(clientset kubernetes.Interface) (*Checker, error) {
	podName := os.Getenv("HOSTNAME")
	if podName == "" {
		return nil, fmt.Errorf("HOSTNAME environment variable not set")
	}

	podNamespace := os.Getenv("POD_NAMESPACE")
	if podNamespace == "" {
		// Try to read from service account namespace
		if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			podNamespace = string(data)
		}
	}
	if podNamespace == "" {
		return nil, fmt.Errorf("POD_NAMESPACE environment variable not set and cannot read from service account")
	}

	checkInterval := DefaultCheckInterval
	if intervalStr := os.Getenv("LEADER_CHECK_INTERVAL"); intervalStr != "" {
		if parsed, err := time.ParseDuration(intervalStr); err == nil && parsed > 0 {
			checkInterval = parsed
		}
	}

	return &Checker{
		clientset:     clientset,
		podName:       podName,
		podNamespace:  podNamespace,
		checkInterval: checkInterval,
	}, nil
}

// IsLeader checks if the current pod is the leader
func (c *Checker) IsLeader(ctx context.Context) (bool, error) {
	// Use cached result if check was recent
	now := time.Now()
	if now.Sub(c.lastCheck) < c.checkInterval && c.lastRole != "" {
		return c.lastRole == RoleLeader, nil
	}

	// Fetch pod to check annotation
	pod, err := c.clientset.CoreV1().Pods(c.podNamespace).Get(ctx, c.podName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get pod %s/%s: %w", c.podNamespace, c.podName, err)
	}

	// Check zen-lead/role annotation
	role := pod.Annotations[AnnotationRole]
	if role == "" {
		// No annotation means not participating in leader election
		// Default to follower behavior
		c.lastRole = RoleFollower
		c.lastCheck = now
		return false, nil
	}

	c.lastRole = role
	c.lastCheck = now
	return role == RoleLeader, nil
}

// WatchLeader watches for leader status changes and calls the callback
func (c *Checker) WatchLeader(ctx context.Context, onLeaderChange func(isLeader bool)) error {
	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	var lastLeaderState bool
	firstCheck := true

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			isLeader, err := c.IsLeader(ctx)
			if err != nil {
				logger.Warn("Failed to check leader status",
					logger.Fields{
						Component: "leader",
						Operation: "check",
						Error:     err,
					})
				continue
			}

			// Call callback on first check or when state changes
			if firstCheck || isLeader != lastLeaderState {
				onLeaderChange(isLeader)
				lastLeaderState = isLeader
				firstCheck = false

				logger.Info("Leader status changed",
					logger.Fields{
						Component: "leader",
						Operation: "status_change",
						Additional: map[string]interface{}{
							"is_leader": isLeader,
							"pod":       c.podName,
						},
					})
			}
		}
	}
}

// GetRole returns the current role (leader/follower)
func (c *Checker) GetRole(ctx context.Context) (string, error) {
	isLeader, err := c.IsLeader(ctx)
	if err != nil {
		return "", err
	}
	if isLeader {
		return RoleLeader, nil
	}
	return RoleFollower, nil
}

