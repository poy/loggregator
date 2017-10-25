package trafficcontroller_test

import (
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TrafficController Health Endpoint", func() {

	It("returns health metrics", func() {
		resp, err := http.Get(fmt.Sprintf("http://%s:11111/health", localIPAddress))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(body).To(ContainSubstring("firehoseStreamCount"))
		Expect(body).To(ContainSubstring("appStreamCount"))
		Expect(body).To(ContainSubstring("slowConsumerCount"))
	})
})