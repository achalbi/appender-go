# Appender Go Microservice

This microservice is a simple HTTP server that appends its POD_NAME to an input string and (eventually) sends the result to a target URL.

## Environment Variables

*   **`POD_NAME`**: The name of the pod where the service is running. This is primarily used for identifying the instance. In a Kubernetes deployment, this should be populated using the Downward API. Defaults to `unknown_pod`.
*   **`TARGET_URL`**: The URL where the appended string will be sent via a POST request. Defaults to `http://localhost:8080/target`.
*   **`PORT`**: The port the service will listen on. Defaults to `8080`.

## Building Locally

To build the executable locally, run the following command:


This is a web server that announces whether or not a particular Go version has been tagged.