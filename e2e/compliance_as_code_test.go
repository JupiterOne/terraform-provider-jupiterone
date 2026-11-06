package e2e

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	importPollAttempts = 60
	importPollInterval = 10 * time.Second
)

// TestComplianceAsCode runs the compliance-as-code workflow with real terraform against a dev account.
func TestComplianceAsCode(t *testing.T) {
	api := newGraphQLClient(requireDevAccount(t))

	runID := strconv.FormatInt(time.Now().Unix(), 10)
	prefix := "tfe2e-" + runID
	workDir := test_structure.CopyTerraformFolderToTemp(t, "fixtures", "compliance-as-code")
	options := &terraform.Options{
		TerraformDir: workDir,
		Vars:         map[string]interface{}{"run_id": runID},
		EnvVars:      map[string]string{"TF_CLI_CONFIG_FILE": devOverridesConfig(t, buildProvider(t))},
		NoColor:      true,
	}

	defer terraform.Destroy(t, options)
	terraform.Apply(t, options)
	require.Equal(t, 0, terraform.PlanExitCode(t, options), "plan after apply must be empty")

	mfa, err := api.control(terraform.OutputRequired(t, options, "mfa_control_id"))
	require.NoError(t, err)
	assert.Len(t, mfa.ControlRequirements.Items, 2, "one control satisfies two requirements")

	reusedID := terraform.OutputRequired(t, options, "reused_control_id")
	reusedSourceID := terraform.OutputRequired(t, options, "reused_control_source_id")

	frameworkName := prefix + " imported framework"
	require.NoError(t, api.importFrameworkJSON(frameworkName, frameworkJSON(frameworkName, prefix, reusedSourceID)))
	importedFrameworkID := waitForImportedFramework(t, api, frameworkName)

	adopted := false
	t.Cleanup(func() {
		if !adopted {
			deleteUnadoptedImport(t, api, importedFrameworkID)
		}
	})

	requirements, err := api.frameworkRequirements(importedFrameworkID)
	require.NoError(t, err)
	require.Len(t, requirements, 1)
	controls := requirements[0].Controls.Items
	require.Len(t, controls, 2, "imported requirement links the reused control and one new control")

	newControlID := ""
	reusedLinked := 0
	for _, control := range controls {
		if control.SourceID == reusedSourceID {
			reusedLinked++
			assert.Equal(t, reusedID, control.ID, "import must link the terraform-managed control, not a copy")
			continue
		}
		newControlID = control.ID
	}
	assert.Equal(t, 1, reusedLinked, "exactly one control carries the reused source_id")
	require.NotEmpty(t, newControlID, "import creates the control that has no matching source_id")

	reused, err := api.control(reusedID)
	require.NoError(t, err)
	assert.Equal(t, "LIVE", reused.Control.State, "reused control stays LIVE")
	assert.Len(t, reused.Control.ControlTests, 1, "import adds no control tests to the reused control")

	require.Equal(t, 0, terraform.PlanExitCode(t, options), "plan after the JSON import must be empty")

	writeImportBlocks(t, workDir, importedFrameworkID, requirements[0].ID, newControlID)
	terraform.RunTerraformCommand(t, options, terraform.FormatArgs(options, "plan", "-input=false", "-generate-config-out=generated.tf")...)
	applyOutput := terraform.Apply(t, options)
	adopted = true
	assert.Contains(t, applyOutput, "Resources: 3 imported, 0 added, 0 changed, 0 destroyed.")
	require.Equal(t, 0, terraform.PlanExitCode(t, options), "plan after adopting the import must be empty")
}

func buildProvider(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	build := exec.Command("go", "build", "-buildvcs=false", "-o", filepath.Join(binDir, "terraform-provider-jupiterone"), ".")
	build.Dir = ".."
	output, err := build.CombinedOutput()
	require.NoError(t, err, "build provider: %s", output)

	return binDir
}

func devOverridesConfig(t *testing.T, binDir string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "terraformrc")
	config := fmt.Sprintf(`provider_installation {
  dev_overrides {
    "JupiterOne/jupiterone" = %q
  }
  direct {}
}
`, binDir)
	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))

	return path
}

func frameworkJSON(name, prefix, reusedSourceID string) map[string]any {
	return map[string]any{
		"name":        name,
		"description": "e2e: framework imported from JSON",
		"requirements": []map[string]any{{
			"displayName": prefix + " imported requirement",
			"description": "requirement created by the JSON import",
			"controls": []map[string]any{
				{"displayName": prefix + " least privilege", "sourceId": reusedSourceID},
				{
					"displayName": prefix + " imported control",
					"description": "control created by the JSON import",
					"state":       "LIVE",
				},
			},
		}},
	}
}

func waitForImportedFramework(t *testing.T, api *graphQLClient, name string) string {
	t.Helper()

	return retry.DoWithRetry(t, "wait for framework import "+name, importPollAttempts, importPollInterval, func() (string, error) {
		job, err := api.inProgressImportJob(name)
		if err != nil {
			return "", err
		}
		if job != nil && job.Status == "FAILED" {
			return "", retry.FatalError{Underlying: fmt.Errorf("framework import failed: %s", job.Message)}
		}
		if job != nil && job.Status == "IN_PROGRESS" {
			return "", errors.New("framework import still in progress")
		}

		id, err := api.frameworkIDByName(name)
		if err != nil {
			return "", err
		}
		if id == "" {
			return "", errors.New("imported framework not visible yet")
		}
		return id, nil
	})
}

func writeImportBlocks(t *testing.T, dir, frameworkID, requirementID, controlID string) {
	t.Helper()

	blocks := []string{
		importBlock("jupiterone_control_framework.imported", frameworkID),
		importBlock("jupiterone_control_framework_requirement.imported", requirementID),
		importBlock("jupiterone_control.imported", controlID),
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "imports.tf"), []byte(strings.Join(blocks, "\n")), 0o600))
}

func importBlock(to, id string) string {
	return fmt.Sprintf("import {\n  to = %s\n  id = %q\n}\n", to, id)
}

func deleteUnadoptedImport(t *testing.T, api *graphQLClient, frameworkID string) {
	t.Helper()

	requirements, err := api.frameworkRequirements(frameworkID)
	if err != nil {
		t.Errorf("read unadopted framework %s for cleanup: %v", frameworkID, err)
	}
	for _, requirement := range requirements {
		for _, control := range requirement.Controls.Items {
			if !strings.HasPrefix(control.ID, "ccm_c_JSON_") {
				continue
			}
			if err := api.deleteControl(control.ID); err != nil {
				t.Errorf("delete imported control %s: %v", control.ID, err)
			}
		}
	}
	if err := api.deleteFramework(frameworkID); err != nil {
		t.Errorf("delete imported framework %s: %v", frameworkID, err)
	}
}
