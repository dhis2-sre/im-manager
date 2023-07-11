// Package inttest enables writing of integration tests. Setup Docker containers for dependencies
// like PostgreSQL, RabbitMQ and AWS S3 (using localstack). Every setup function ensures the
// container is ready before returning, ensures resources are cleaned up after the tests are
// finished and return a client ready to interact with the container.
package inttest
