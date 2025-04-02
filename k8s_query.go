package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sInfo struct {
	Timestamp   string                 `json:"timestamp"`
	Namespaces  []NamespaceInfo       `json:"namespaces"`
	Nodes       []NodeInfo            `json:"nodes"`
}

type NamespaceInfo struct {
	Name string     `json:"name"`
	Pods []PodInfo  `json:"pods"`
}

type PodInfo struct {
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	NodeName  string         `json:"nodeName"`
	Containers []ContainerInfo `json:"containers"`
}

type ContainerInfo struct {
	Name      string         `json:"name"`
	Image     string         `json:"image"`
	EnvVars   []EnvVarInfo   `json:"environmentVariables"`
}

type EnvVarInfo struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
	Source string `json:"source,omitempty"`
}

type NodeInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type GitConfig struct {
	RepoURL      string
	Branch       string
	Username     string
	Password     string
	CommitMessage string
}

func writeCSV(k8sInfo K8sInfo, outputDir string) error {
	// Create CSV files for different types of information
	files := map[string]*os.File{
		"pods.csv":           nil,
		"containers.csv":     nil,
		"env_vars.csv":       nil,
		"nodes.csv":          nil,
	}

	// Create and open all CSV files
	for filename := range files {
		filePath := filepath.Join(outputDir, filename)
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("error creating %s: %v", filePath, err)
		}
		files[filename] = file
		defer file.Close()
	}

	// Create CSV writers
	writers := make(map[string]*csv.Writer)
	for filename, file := range files {
		writers[filename] = csv.NewWriter(file)
		defer writers[filename].Flush()
	}

	// Write headers
	writers["pods.csv"].Write([]string{"Namespace", "Pod Name", "Status", "Node Name"})
	writers["containers.csv"].Write([]string{"Namespace", "Pod Name", "Container Name", "Image"})
	writers["env_vars.csv"].Write([]string{"Namespace", "Pod Name", "Container Name", "Variable Name", "Value", "Source"})
	writers["nodes.csv"].Write([]string{"Node Name", "Status"})

	// Write pod and container data
	for _, ns := range k8sInfo.Namespaces {
		for _, pod := range ns.Pods {
			// Write pod information
			writers["pods.csv"].Write([]string{
				ns.Name,
				pod.Name,
				pod.Status,
				pod.NodeName,
			})

			// Write container information
			for _, container := range pod.Containers {
				writers["containers.csv"].Write([]string{
					ns.Name,
					pod.Name,
					container.Name,
					container.Image,
				})

				// Write environment variables
				for _, env := range container.EnvVars {
					writers["env_vars.csv"].Write([]string{
						ns.Name,
						pod.Name,
						container.Name,
						env.Name,
						env.Value,
						env.Source,
					})
				}
			}
		}
	}

	// Write node information
	for _, node := range k8sInfo.Nodes {
		writers["nodes.csv"].Write([]string{
			node.Name,
			node.Status,
		})
	}

	return nil
}

func gitPush(outputDir string, config GitConfig) error {
	// Initialize or open repository
	repo, err := git.PlainInit(outputDir, false)
	if err != nil {
		if err != git.ErrRepositoryAlreadyExists {
			return fmt.Errorf("error initializing repository: %v", err)
		}
		// If repo exists, open it
		repo, err = git.PlainOpen(outputDir)
		if err != nil {
			return fmt.Errorf("error opening repository: %v", err)
		}
	}

	// Get the worktree
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("error getting worktree: %v", err)
	}

	// Add all files
	_, err = w.Add(".")
	if err != nil {
		return fmt.Errorf("error adding files: %v", err)
	}

	// Create commit
	_, err = w.Commit(config.CommitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  config.Username,
			Email: fmt.Sprintf("%s@users.noreply.github.com", config.Username),
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("error creating commit: %v", err)
	}

	// Set up remote
	_, err = repo.Remote("origin")
	if err == git.ErrRemoteNotFound {
		remoteURL := fmt.Sprintf("https://%s", config.RepoURL)
		_, err = repo.CreateRemote(&git.RemoteConfig{
			Name: "origin",
			URLs: []string{remoteURL},
		})
		if err != nil {
			return fmt.Errorf("error creating remote: %v", err)
		}
	} else if err != nil {
		return fmt.Errorf("error getting remote: %v", err)
	}

	// Set up authentication
	auth := &http.BasicAuth{
		Username: config.Username,
		Password: config.Password,
	}

	// Create branch if it doesn't exist
	branchRef := plumbing.NewBranchReferenceName(config.Branch)
	_, err = repo.Reference(branchRef, false)
	if err == plumbing.ErrReferenceNotFound {
		headRef, err := repo.Head()
		if err != nil {
			return fmt.Errorf("error getting HEAD reference: %v", err)
		}
		ref := plumbing.NewHashReference(branchRef, headRef.Hash())
		err = repo.Storer.SetReference(ref)
		if err != nil {
			return fmt.Errorf("error creating branch: %v", err)
		}
	}

	// Push changes
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", config.Branch, config.Branch)),
		},
		Auth: auth,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("error pushing changes: %v", err)
	}

	return nil
}

func loadEnvFile() error {
	// Read .env file
	data, err := os.ReadFile(".env")
	if err != nil {
		// If .env file doesn't exist, that's okay
		return nil
	}

	// Parse and set environment variables
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, `"'`)

		// Skip empty values
		if value == "" {
			continue
		}

		// Set environment variable
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("error setting environment variable %s: %v", key, err)
		}
	}

	return nil
}

func main() {
	// Load environment variables from .env file
	if err := loadEnvFile(); err != nil {
		fmt.Printf("Warning: Error loading .env file: %v\n", err)
	}

	// Create output directory
	outputDir := "k8s-data"
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Get the kubeconfig file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting home directory: %v\n", err)
		os.Exit(1)
	}
	kubeconfig := filepath.Join(homeDir, ".kube", "config")

	// Create the config from kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating clientset: %v\n", err)
		os.Exit(1)
	}

	// Initialize the K8sInfo structure
	k8sInfo := K8sInfo{
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Get list of namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing namespaces: %v\n", err)
		os.Exit(1)
	}

	// Process namespaces and pods
	for _, ns := range namespaces.Items {
		namespaceInfo := NamespaceInfo{
			Name: ns.Name,
		}

		// Get pods in the namespace
		pods, err := clientset.CoreV1().Pods(ns.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Error listing pods in namespace %s: %v\n", ns.Name, err)
			continue
		}

		// Process pods
		for _, pod := range pods.Items {
			podInfo := PodInfo{
				Name:     pod.Name,
				Status:   string(pod.Status.Phase),
				NodeName: pod.Spec.NodeName,
			}

			// Process containers
			for _, container := range pod.Spec.Containers {
				containerInfo := ContainerInfo{
					Name:  container.Name,
					Image: container.Image,
				}

				// Process environment variables
				if len(container.Env) > 0 {
					for _, env := range container.Env {
						envInfo := EnvVarInfo{
							Name: env.Name,
						}
						if env.ValueFrom != nil {
							if env.ValueFrom.ConfigMapKeyRef != nil {
								envInfo.Source = fmt.Sprintf("ConfigMap: %s", env.ValueFrom.ConfigMapKeyRef.Name)
							} else if env.ValueFrom.SecretKeyRef != nil {
								envInfo.Source = fmt.Sprintf("Secret: %s", env.ValueFrom.SecretKeyRef.Name)
							}
						} else {
							envInfo.Value = env.Value
						}
						containerInfo.EnvVars = append(containerInfo.EnvVars, envInfo)
					}
				}

				podInfo.Containers = append(podInfo.Containers, containerInfo)
			}

			namespaceInfo.Pods = append(namespaceInfo.Pods, podInfo)
		}

		k8sInfo.Namespaces = append(k8sInfo.Namespaces, namespaceInfo)
	}

	// Get list of nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing nodes: %v\n", err)
		os.Exit(1)
	}

	// Process nodes
	for _, node := range nodes.Items {
		nodeInfo := NodeInfo{
			Name:   node.Name,
			Status: string(node.Status.Conditions[len(node.Status.Conditions)-1].Type),
		}
		k8sInfo.Nodes = append(k8sInfo.Nodes, nodeInfo)
	}

	// Marshal to JSON with pretty printing
	jsonData, err := json.MarshalIndent(k8sInfo, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling to JSON: %v\n", err)
		os.Exit(1)
	}

	// Write to JSON file
	outputFile := filepath.Join(outputDir, "k8s_info.json")
	err = os.WriteFile(outputFile, jsonData, 0644)
	if err != nil {
		fmt.Printf("Error writing to JSON file: %v\n", err)
		os.Exit(1)
	}

	// Write to CSV files
	err = writeCSV(k8sInfo, outputDir)
	if err != nil {
		fmt.Printf("Error writing to CSV files: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Kubernetes information has been written to %s directory:\n", outputDir)
	fmt.Printf("- JSON: %s\n", filepath.Join(outputDir, "k8s_info.json"))
	fmt.Printf("- CSV files: %s\n", filepath.Join(outputDir, "pods.csv"))
	fmt.Printf("            %s\n", filepath.Join(outputDir, "containers.csv"))
	fmt.Printf("            %s\n", filepath.Join(outputDir, "env_vars.csv"))
	fmt.Printf("            %s\n", filepath.Join(outputDir, "nodes.csv"))

	// Git repository configuration
	gitConfig := GitConfig{
		RepoURL:      os.Getenv("K8S_GIT_REPO"),
		Branch:       os.Getenv("K8S_GIT_BRANCH"),
		Username:     os.Getenv("K8S_GIT_USERNAME"),
		Password:     os.Getenv("K8S_GIT_PASSWORD"),
		CommitMessage: fmt.Sprintf("Kubernetes cluster snapshot - %s", time.Now().Format("20060102-150405")),
	}

	// Push to git repository if configuration is provided
	if gitConfig.RepoURL != "" && gitConfig.Username != "" && gitConfig.Password != "" {
		if gitConfig.Branch == "" {
			gitConfig.Branch = "main"
		}

		fmt.Printf("\nPushing data to Git repository...\n")
		err = gitPush(outputDir, gitConfig)
		if err != nil {
			fmt.Printf("Error pushing to Git repository: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully pushed data to Git repository\n")
	} else {
		fmt.Printf("\nSkipping Git push - repository configuration not provided\n")
		fmt.Printf("Set the following environment variables to enable Git push:\n")
		fmt.Printf("- K8S_GIT_REPO: Repository URL (e.g., github.com/username/repo.git)\n")
		fmt.Printf("- K8S_GIT_USERNAME: Repository username\n")
		fmt.Printf("- K8S_GIT_PASSWORD: Repository password/token\n")
		fmt.Printf("- K8S_GIT_BRANCH: Branch name (optional, defaults to 'main')\n")
	}
} 