package dopplerforwarder_test

import (
	"errors"
	"metron/writers/dopplerforwarder"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DopplerForwarder", func() {
	var (
		sender      *fake.FakeMetricSender
		clientPool  *mockClientPool
		client      *mockClient
		logger      *gosteno.Logger
		forwarder   *dopplerforwarder.DopplerForwarder
		fakeWrapper *mockNetworkWrapper
		mockRetrier *mockRetrier
		retrier     dopplerforwarder.Retrier
		message     []byte
	)

	BeforeEach(func() {
		message = []byte("I am a message!")

		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))

		client = newMockClient()
		clientPool = newMockClientPool()
		clientPool.RandomClientOutput.client <- client
		close(clientPool.RandomClientOutput.err)

		logger = loggertesthelper.Logger()
		loggertesthelper.TestLoggerSink.Clear()

		fakeWrapper = newMockNetworkWrapper()
		close(fakeWrapper.WriteOutput.ret0)
	})

	JustBeforeEach(func() {
		if mockRetrier != nil {
			retrier = mockRetrier
		}
		forwarder = dopplerforwarder.New(fakeWrapper, clientPool, retrier, logger)
	})

	Context("client selection", func() {
		It("selects a random client", func() {
			forwarder.Write(message)
			Eventually(fakeWrapper.WriteInput.client).Should(Receive(Equal(client)))
			Eventually(fakeWrapper.WriteInput.message).Should(Receive(Equal(message)))
		})

		Context("when selecting a client errors", func() {
			It("logs an error and returns", func() {
				clientPool.RandomClientOutput.err = make(chan error, 1)
				clientPool.RandomClientOutput.err <- errors.New("boom")
				forwarder.Write(message)

				Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("failed to pick a client"))
				Consistently(fakeWrapper.WriteCalled).ShouldNot(Receive())
			})
		})

		Context("when networkWrapper write fails", func() {
			It("logs an error and returns", func() {
				fakeWrapper.WriteOutput.ret0 = make(chan error, 1)
				fakeWrapper.WriteOutput.ret0 <- errors.New("boom")
				forwarder.Write(message)

				Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("failed to write message"))
			})
		})

		Context("with a retrier", func() {
			BeforeEach(func() {
				mockRetrier = newMockRetrier()
				close(mockRetrier.RetryOutput.ret0)
			})

			It("does not retry if a client cannot be found", func() {
				clientPool.RandomClientOutput.err = make(chan error, 1)
				clientPool.RandomClientOutput.err <- errors.New("boom")
				forwarder.Write(message)

				Consistently(mockRetrier.RetryCalled).ShouldNot(Receive())
			})

			It("retries if a client write errors", func() {
				fakeWrapper.WriteOutput.ret0 = make(chan error, 1)
				fakeWrapper.WriteOutput.ret0 <- errors.New("boom")
				forwarder.Write(message)

				Eventually(mockRetrier.RetryCalled).Should(Receive())
				Expect(mockRetrier.RetryInput.message).To(Receive(Equal(message)))
			})

			It("logs an error if the retrier errors", func() {
				fakeWrapper.WriteOutput.ret0 = make(chan error, 1)
				fakeWrapper.WriteOutput.ret0 <- errors.New("boom")

				mockRetrier.RetryOutput.ret0 = make(chan error, 1)
				mockRetrier.RetryOutput.ret0 <- errors.New("boom")
				forwarder.Write(message)

				Eventually(mockRetrier.RetryCalled).Should(Receive())
				Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("failed to retry message"))
				Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.retryCount") }).Should(BeZero())
			})

			It("increments retryCount", func() {
				fakeWrapper.WriteOutput.ret0 = make(chan error, 1)
				fakeWrapper.WriteOutput.ret0 <- errors.New("boom")
				forwarder.Write(message)

				Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.retryCount") }).Should(BeEquivalentTo(1))
			})
		})
		
		Context("metrics", func() {
			It("emits the sentMessages metric", func() {
				forwarder.Write(message)
				Eventually(fakeWrapper.WriteInput.message).Should(Receive(Equal(message)))
				Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(1))
			})
		})
	})

	Describe("Weight", func(){
		BeforeEach(func() {
			clientPool = newMockClientPool()
			clientPool.SizeOutput.ret0 <- 10
		})
	    It("returns the size of the client pool", func() {
			Expect(forwarder.Weight()).To(Equal(10))
	    })
	})
})
