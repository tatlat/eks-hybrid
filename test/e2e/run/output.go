package run

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type E2EOutput struct {
	ArtifactsBucket     string
	ArtifactsBucketPath string
	ClusterRegion       string
}

func (e *E2EOutput) PrintJSON(e2eResult E2EResult) error {
	e2eResult.NodeadmE2eTestResultJSON = true
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(e2eResult); err != nil {
		return fmt.Errorf("encoding e2e result: %w", err)
	}
	fmt.Printf("%s\n", buf.String())
	return nil
}

func (e *E2EOutput) PrintText(e2eResult E2EResult) {
	for _, phase := range e2eResult.Phases {
		if phase.Status == "success" {
			continue
		}
		fmt.Printf("Phase: %s Failure: %s Status: %s\n", phase.Name, phase.FailureMessage, phase.Status)
	}
	fmt.Printf("%d/%d Tests ran\n\n", e2eResult.TestRan, e2eResult.TotalTests)
	fmt.Printf("%d/%d Tests failed\n", e2eResult.TestFailed, e2eResult.TestRan)

	fmt.Printf("Cluster Artifacts Bucket Path: %s\n", e2eResult.ArtifactsBucketPath)
	fmt.Printf("Full Ginkgo Test Log: %s\n", e2eResult.GinkgoLog)
	fmt.Printf("Cluster Setup Log: %s\n", e2eResult.SetupLog)
	fmt.Printf("Cluster Cleanup Log: %s\n\n", e2eResult.CleanupLog)

	var testPhase Phase
	for _, phase := range e2eResult.Phases {
		if phase.Name == phaseNameExecuteTests {
			testPhase = phase
			break
		}
	}

	if testPhase.Status == "success" {
		fmt.Printf("All tests passed\n")
		return
	}

	if len(e2eResult.FailedTests) == 0 {
		// tests did not run
		return
	}

	fmt.Printf("Failed tests:\n")
	for _, failedTest := range e2eResult.FailedTests {
		fmt.Printf("\n\t[%s] - %s\n", failedTest.Name, failedTest.State)
		fmt.Printf("\tTest Artifacts Bucket Path: %s\n", failedTest.GinkgoLog[:strings.LastIndex(failedTest.GinkgoLog, "/")+1])
		fmt.Printf("\tTest Ginkgo Log: %s\n", failedTest.GinkgoLog)
		fmt.Printf("\tTest Serial Log: %s\n", failedTest.SerialLog)
		fmt.Printf("\tTest Log Collector Bundle: %s\n", failedTest.CollectorLogsBundle)
		if failedTest.FailureMessage != "" {
			fmt.Printf("\t%s\n", strings.ReplaceAll(failedTest.FailureMessage, "\n", "\n\t"))
		}
		fmt.Printf("\n")
	}
}
