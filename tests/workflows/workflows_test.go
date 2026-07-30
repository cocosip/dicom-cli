package workflows_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type workflow struct {
	On          workflowTriggers  `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]job    `yaml:"jobs"`
}

type workflowTriggers struct {
	Push struct {
		Branches []string `yaml:"branches"`
		Tags     []string `yaml:"tags"`
	} `yaml:"push"`
}

type job struct {
	RunsOn   string `yaml:"runs-on"`
	Strategy struct {
		Matrix struct {
			OS []string `yaml:"os"`
		} `yaml:"matrix"`
	} `yaml:"strategy"`
	Steps []step `yaml:"steps"`
}

type step struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
}

func TestCIWorkflowRunsOnlyForMasterAcrossNativeRunners(t *testing.T) {
	_, workflow := loadWorkflow(t, "ci.yml")
	if !reflect.DeepEqual(workflow.On.Push.Branches, []string{"master"}) {
		t.Fatalf("CI branches = %v, want [master]", workflow.On.Push.Branches)
	}
	if len(workflow.On.Push.Tags) != 0 {
		t.Fatalf("CI tags = %v, want none", workflow.On.Push.Tags)
	}
	if workflow.Permissions["contents"] != "read" {
		t.Fatalf("CI contents permission = %q, want read", workflow.Permissions["contents"])
	}
	verify := workflow.Jobs["verify"]
	if verify.RunsOn != "${{ matrix.os }}" {
		t.Fatalf("CI runner = %q, want matrix runner", verify.RunsOn)
	}
	if !reflect.DeepEqual(verify.Strategy.Matrix.OS, []string{"windows-latest", "ubuntu-latest", "macos-latest"}) {
		t.Fatalf("CI runner matrix = %v, want Windows/Linux/macOS", verify.Strategy.Matrix.OS)
	}
	for name, fragment := range map[string]string{
		"Check formatting": "gofmt -l",
		"Run go vet":       "go vet ./...",
		"Run tests":        "go test ./...",
		"Build dicom-cli":  "go build ./cmd/dicom-cli",
	} {
		if !strings.Contains(stepRun(t, verify, name), fragment) {
			t.Fatalf("CI step %q does not contain %q", name, fragment)
		}
	}
}

func TestReleaseWorkflowRunsOnlyForVersionTags(t *testing.T) {
	_, workflow := loadWorkflow(t, "release.yml")
	if len(workflow.On.Push.Branches) != 0 {
		t.Fatalf("release branches = %v, want none", workflow.On.Push.Branches)
	}
	if !reflect.DeepEqual(workflow.On.Push.Tags, []string{"v*"}) {
		t.Fatalf("release tags = %v, want [v*]", workflow.On.Push.Tags)
	}
	if workflow.Permissions["contents"] != "write" {
		t.Fatalf("release contents permission = %q, want write", workflow.Permissions["contents"])
	}
	release := workflow.Jobs["release"]
	if release.RunsOn != "ubuntu-latest" {
		t.Fatalf("release runner = %q, want ubuntu-latest", release.RunsOn)
	}
	if !strings.Contains(stepRun(t, release, "Build release archives"), "go run ./cmd/release-packager --version \"${GITHUB_REF_NAME#v}\" --output dist") {
		t.Fatal("release build step does not pass the tag version to release-packager")
	}
	if !strings.Contains(stepRun(t, release, "Smoke test Linux archive"), "dicom-cli_${GITHUB_REF_NAME#v}_linux_amd64") {
		t.Fatal("release smoke test does not execute the Linux AMD64 archive")
	}
	createRelease := stepRun(t, release, "Create GitHub Release")
	for _, asset := range []string{
		"dicom-cli_${GITHUB_REF_NAME#v}_windows_amd64.zip",
		"dicom-cli_${GITHUB_REF_NAME#v}_linux_amd64.tar.gz",
		"dicom-cli_${GITHUB_REF_NAME#v}_linux_arm64.tar.gz",
		"dicom-cli_${GITHUB_REF_NAME#v}_darwin_amd64.tar.gz",
		"dicom-cli_${GITHUB_REF_NAME#v}_darwin_arm64.tar.gz",
	} {
		if !strings.Contains(createRelease, asset) {
			t.Fatalf("release upload command does not contain %q", asset)
		}
	}
}

func stepRun(t *testing.T, job job, name string) string {
	t.Helper()
	for _, step := range job.Steps {
		if step.Name == name {
			return step.Run
		}
	}
	t.Fatalf("workflow step %q is missing", name)
	return ""
}

func loadWorkflow(t *testing.T, name string) (string, workflow) {
	t.Helper()
	path := filepath.Join("..", "..", ".github", "workflows", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed workflow
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return string(content), parsed
}
