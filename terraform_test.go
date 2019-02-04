package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/aws"
	http_helper "github.com/gruntwork-io/terratest/modules/http-helper"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/ssh"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
)

func TestTerraformHttp(t *testing.T) {
	t.Parallel()

	terraformOptions := &terraform.Options{
		// The path to where our Terraform code is located
		TerraformDir: "./terraform",
	}

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	url := terraform.Output(t, terraformOptions, "alb_url")

	maxRetries := 30
	timeBetweenRetries := 5 * time.Second

	// Verify that we get back a 200 OK with the expected instanceText
	http_helper.HttpGetWithRetry(t, url, 200, "Hello!!!", maxRetries, timeBetweenRetries)
}

func TestTerraformSsh(t *testing.T) {
	t.Parallel()

	terraformDir := test_structure.CopyTerraformFolderToTemp(t, "../", "terraform")

	defer test_structure.RunTestStage(t, "teardown", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, terraformDir)
		terraform.Destroy(t, terraformOptions)

		keyPair := test_structure.LoadEc2KeyPair(t, terraformDir)
		aws.DeleteEC2KeyPair(t, keyPair)
	})

	test_structure.RunTestStage(t, "setup", func() {

		keyPair := aws.CreateAndImportEC2KeyPair(t, "ap-northeast-1", "terratest-ssh-key")
		terraformOptions := &terraform.Options{
			// The path to where our Terraform code is located
			TerraformDir: "./terraform",
			Vars: map[string]interface{}{
				"key_pair_name": keyPair.Name,
			},
		}

		// Save the options and key pair so later test stages can use them
		test_structure.SaveTerraformOptions(t, terraformDir, terraformOptions)
		test_structure.SaveEc2KeyPair(t, terraformDir, keyPair)

		// This will run `terraform init` and `terraform apply` and fail the test if there are any errors
		terraform.InitAndApply(t, terraformOptions)
	})

	test_structure.RunTestStage(t, "validate", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, terraformDir)
		keyPair := test_structure.LoadEc2KeyPair(t, terraformDir)

		publicIP := terraform.Output(t, terraformOptions, "ssh_ip_address")

		publicHost := ssh.Host{
			Hostname:    publicIP,
			SshKeyPair:  keyPair.KeyPair,
			SshUserName: "ec2-user",
		}

		maxRetries := 30
		timeBetweenRetries := 5 * time.Second
		description := fmt.Sprintf("SSH to public host %s", publicIP)

		expectedText := "Hello!!!"
		command := fmt.Sprintf("echo -n '%s'", expectedText)

		retry.DoWithRetry(t, description, maxRetries, timeBetweenRetries, func() (string, error) {
			actualText, err := ssh.CheckSshCommandE(t, publicHost, command)

			if err != nil {
				fmt.Println("errrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrr")
				fmt.Println(err)
				return "", err
			}

			if strings.TrimSpace(actualText) != expectedText {
				fmt.Println("errrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrr")
				fmt.Println(actualText)
				return "", fmt.Errorf("Expected SSH command to return '%s' but got '%s'", expectedText, actualText)
			}

			return "", nil
		})
	})

}
