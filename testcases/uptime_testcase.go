package testcases

import (
	"log"
	"net/http"
	"path"
	"time"

	. "github.com/cloudfoundry-incubator/disaster-recovery-acceptance-tests/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const CURL_ERROR_FOR_404 = 22

type AppUptimeTestCase struct {
	uniqueTestID            string
	stopCheckingAppAlive    chan<- bool
	stopCheckingAPIGoesDown chan<- bool
	valueApiWasDown         <-chan bool
}

func NewAppUptimeTestCase() *AppUptimeTestCase {
	id := RandomStringNumber()
	return &AppUptimeTestCase{uniqueTestID: id}
}

func (tc *AppUptimeTestCase) BeforeBackup(config Config) {
	RunCommandSuccessfully("cf login --skip-ssl-validation -a", config.DeploymentToBackup.ApiUrl, "-u", config.DeploymentToBackup.AdminUsername, "-p", config.DeploymentToBackup.AdminPassword)
	RunCommandSuccessfully("cf create-org acceptance-test-org-" + tc.uniqueTestID)
	RunCommandSuccessfully("cf create-space acceptance-test-space-" + tc.uniqueTestID + " -o acceptance-test-org-" + tc.uniqueTestID)
	RunCommandSuccessfully("cf target -o acceptance-test-org-" + tc.uniqueTestID + " -s acceptance-test-space-" + tc.uniqueTestID)
	var testAppFixturePath = path.Join(CurrentTestDir(), "/../fixtures/test_app/")
	RunCommandSuccessfully("cf push test_app_" + tc.uniqueTestID + " -p " + testAppFixturePath)

	By("checking the app stays up")
	appUrl := GetAppUrl("test_app_" + tc.uniqueTestID)
	tc.stopCheckingAppAlive = checkAppRemainsAlive(appUrl)
	tc.stopCheckingAPIGoesDown, tc.valueApiWasDown = checkApiGoesDown(config.DeploymentToBackup.ApiUrl)
}

func (tc *AppUptimeTestCase) AfterBackup(config Config) {
	By("stopping checking the app")
	log.Println("writing to stopCheckingAppAlive...")
	tc.stopCheckingAppAlive <- true
	log.Println("writing to stopCheckingAPIGoesDown...")
	tc.stopCheckingAPIGoesDown <- true
	log.Println("reading from valueApiWasDown...")
	Expect(<-tc.valueApiWasDown).To(BeTrue(), "expected api to be down, but it isn't")
}

func (tc *AppUptimeTestCase) AfterRestore(config Config) {

}

func (tc *AppUptimeTestCase) Cleanup(config Config) {
	By("cleaning up orgs, spaces and apps")
	RunCommandSuccessfully("cf login --skip-ssl-validation -a", config.DeploymentToBackup.ApiUrl, "-u", config.DeploymentToBackup.AdminUsername, "-p", config.DeploymentToBackup.AdminPassword)
	RunCommandSuccessfully("cf target -o acceptance-test-org-" + tc.uniqueTestID)
	RunCommandSuccessfully("cf delete-space -f acceptance-test-space-" + tc.uniqueTestID)
	RunCommandSuccessfully("cf delete-org -f acceptance-test-org-" + tc.uniqueTestID)
}

func checkApiGoesDown(apiUrl string) (chan<- bool, <-chan bool) {
	doneChannel := make(chan bool)
	valueApiWasDown := make(chan bool)
	ticker := time.NewTicker(1 * time.Second)
	tickerChannel := ticker.C

	go func() {
		var apiWasDown bool
		defer GinkgoRecover()
		for {
			select {
			case <-doneChannel:
				valueApiWasDown <- apiWasDown
				return
			case <-tickerChannel:
				if RunCommand("curl", "-k", "--fail", apiUrl).ExitCode() == CURL_ERROR_FOR_404 {
					apiWasDown = true
					ticker.Stop()
				}
			}
		}
	}()

	return doneChannel, valueApiWasDown
}

func checkAppRemainsAlive(url string) chan<- bool {
	doneChannel := make(chan bool, 1)
	ticker := time.NewTicker(1 * time.Second)
	tickerChannel := ticker.C

	go func() {
		defer GinkgoRecover()
		for {
			select {
			case <-doneChannel:
				ticker.Stop()
				return
			case <-tickerChannel:
				Expect(Get(url).StatusCode).To(Equal(http.StatusOK))
			}
		}
	}()

	return doneChannel
}
