package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"bytes"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	podName    string
	targetURL  string
	listenAddr = ":8080"
)

func init() {
	// Set up logrus
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)

	// Read environment variables
	podName = os.Getenv("POD_NAME")
	if podName == "" {
		podName = "default-instance"
	}

	targetURL = os.Getenv("TARGET_URL")
	if targetURL == "" {
		// Set a default or make it required, depending on your needs.
		// For this example, we'll log a warning and proceed without a target URL.
		logrus.Warn("TARGET_URL environment variable not set. The service will not forward requests.")
	}
}

func main() {
	router := gin.Default()

	// Redirect root to sender.html
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/sender.html")
	})

	router.GET("/health", healthHandler)
	router.POST("/append", appendHandler)

	// Serve static files from the current directory
	router.Static("/", ".")

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: router,
	}

	go func() {
		logrus.Infof("Starting server on %s", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("listen: %s", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be caught, so don't need to add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logrus.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logrus.Fatal("Server forced to shutdown:", err)
	}

	logrus.Info("Server exiting")
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "UP"})
}

type AppendRequest struct {
	Input string `json:"input"`
}

type AppendResponse struct {
	Result string `json:"result"`
}

func appendHandler(c *gin.Context) {
	var req AppendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Error("Failed to bind JSON")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	appendedString := fmt.Sprintf("%s I am Java instance %s", req.Input, podName)
	logrus.WithField("result", appendedString).Info("Appended string")

	if targetURL != "" {
		// Send the result to the target URL
		payload := AppendResponse{Result: appendedString}
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			logrus.WithError(err).Error("Failed to marshal JSON payload for target URL")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process request"})
			return
		}

		resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			logrus.WithError(err).WithField("target_url", targetURL).Error("Failed to send POST request to target URL")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to forward request to target service"})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logrus.WithField("target_url", targetURL).WithField("status_code", resp.StatusCode).Info("Successfully forwarded request to target URL")
			c.JSON(http.StatusOK, gin.H{"message": "Request processed and forwarded"})
			return // Return here after successful forwarding
		} else {
			// Log and return an error for non-2xx status codes from target
			// We are returning StatusBadGateway as we failed to get a successful response from the upstream service
			// You might want to return the exact status code received from the target in some cases, but 502 is a common pattern for upstream errors.
			logrus.WithFields(logrus.Fields{"target_url": targetURL, "status_code": resp.StatusCode}).Error("Target URL returned non-OK status code")
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Target service returned status code %d", resp.StatusCode)})
			return
		}

	}
	// If targetURL is not set, still return 200 OK for successful appending
	// If targetURL was set and the request failed, the handler would have already returned.
	c.JSON(http.StatusOK, gin.H{"message": "Request processed"})
}