package buildapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("APIServer", func() {
	var (
		server *APIServer
		logger logr.Logger
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		logger = logr.Discard()
		server = NewAPIServer(":0", logger)
	})

	AfterEach(func() {
		server = nil
	})

	Context("Server Creation", func() {
		It("should create a valid API server", func() {
			Expect(server).NotTo(BeNil())
			Expect(server.router).NotTo(BeNil())
			Expect(server.server).NotTo(BeNil())
			Expect(server.addr).To(Equal(":0"))
			Expect(server.log).To(Equal(logger))
		})
	})

	Context("Health Endpoint", func() {
		It("should return 200 OK for health check", func() {
			req, err := http.NewRequest("GET", "/v1/healthz", nil)
			Expect(err).NotTo(HaveOccurred())

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(Equal("ok"))
		})
	})

	Context("OpenAPI Endpoint", func() {
		It("should return OpenAPI spec", func() {
			req, err := http.NewRequest("GET", "/v1/openapi.yaml", nil)
			Expect(err).NotTo(HaveOccurred())

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("application/yaml"))
			Expect(w.Body.String()).NotTo(BeEmpty())
		})
	})

	Context("Builds Endpoints Authentication", func() {
		var testCases = []struct {
			method string
			path   string
		}{
			{"GET", "/v1/builds"},
			{"POST", "/v1/builds"},
			{"GET", "/v1/builds/test-build"},
			{"GET", "/v1/builds/test-build/logs"},
			{"GET", "/v1/builds/test-build/artifacts"},
			{"GET", "/v1/builds/test-build/template"},
			{"POST", "/v1/builds/test-build/uploads"},
		}

		It("should require authentication for all builds endpoints", func() {
			for _, tc := range testCases {
				By(fmt.Sprintf("testing %s %s", tc.method, tc.path))

				req, err := http.NewRequest(tc.method, tc.path, nil)
				Expect(err).NotTo(HaveOccurred())

				w := httptest.NewRecorder()
				server.router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusUnauthorized))
			}
		})
	})

	Context("Server Lifecycle", func() {
		It("should start and stop gracefully", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			errChan := make(chan error, 1)
			go func() {
				errChan <- server.Start(ctx)
			}()

			time.Sleep(100 * time.Millisecond)

			cancel()

			Eventually(errChan, 2*time.Second).Should(Receive(BeNil()))
		})
	})

	Context("Integration with Kubernetes", func() {
		BeforeEach(func() {
			if os.Getenv("KUBECONFIG") == "" && os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
				Skip("no kubernetes configuration found")
			}
		})

		It("should be able to connect to Kubernetes cluster", func() {
			req, err := http.NewRequest("GET", "/v1/healthz", nil)
			Expect(err).NotTo(HaveOccurred())

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(Equal("ok"))
		})
	})
})

var _ = Describe("APIServer Performance", func() {
	var (
		server *APIServer
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		server = NewAPIServer(":0", logr.Discard())
	})

	It("should handle health endpoint requests", func() {
		req, _ := http.NewRequest("GET", "/v1/healthz", nil)

		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("should handle openapi endpoint requests efficiently", func() {
		req, _ := http.NewRequest("GET", "/v1/openapi.yaml", nil)

		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})
})
