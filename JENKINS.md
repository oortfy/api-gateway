# Jenkins CI/CD Setup

This document provides instructions for setting up Jenkins CI/CD pipelines for the API Gateway project.

## Prerequisites

- Jenkins server with the following plugins installed:
  - Kubernetes Plugin
  - Pipeline
  - HTML Publisher (for test reports)
  - Git
  - Credentials

- Kubernetes cluster with:
  - A ServiceAccount named 'jenkins' that has permission to create pods
  - Ability to mount the Docker socket from the host (for Docker operations)

## Pipeline Types

The project includes two Jenkinsfiles:

1. `Jenkinsfile` - For continuous integration (building and testing)
2. `Jenkinsfile.release` - For creating releases and publishing Docker images

## Jenkins Kubernetes Setup

The pipelines are designed to run in Kubernetes pods using the Jenkins Kubernetes plugin. This provides several advantages:
- Isolated build environments
- Scalability for concurrent builds
- Better resource utilization

### Kubernetes Plugin Configuration

1. In Jenkins, go to Manage Jenkins > Manage Nodes and Clouds > Configure Clouds
2. Add a new cloud of type Kubernetes
3. Configure the Kubernetes cloud:
   - Kubernetes URL: URL to your Kubernetes API server
   - Kubernetes Namespace: The namespace where Jenkins should create pods
   - Credentials: Select your Kubernetes credentials
   - Jenkins URL: https://jenkins.oortfy.com/ (or your Jenkins URL)

Note that the pod template is defined directly in the Jenkinsfiles, so no additional pod templates need to be configured in Jenkins.

## Setting Up Jenkins Pipelines

### Continuous Integration Pipeline

1. In Jenkins, create a new pipeline job
2. Configure the pipeline to use SCM:
   - Select "Git" as the SCM
   - Enter your repository URL
   - Specify the branches to build (e.g., `*/main`, `*/dev`)
   - Set the Script Path to `Jenkinsfile`
3. Configure build triggers as needed (e.g., poll SCM, webhooks)
4. Save the job

This pipeline will:
- Run code linting with golangci-lint
- Run tests with race detection
- Generate coverage reports
- Build the application binary
- Build a Docker image (without pushing)

### Release Pipeline

1. In Jenkins, create a new pipeline job
2. Configure the pipeline to use SCM:
   - Select "Git" as the SCM
   - Enter your repository URL
   - Specify the branch or tag to build
   - Set the Script Path to `Jenkinsfile.release`
3. Configure this as a parameterized build with:
   - `VERSION` - String parameter for the release version
   - `PUSH_DOCKER` - Boolean parameter to control Docker pushing
4. Save the job

This pipeline will:
- Create release artifacts using GoReleaser
- Build a Docker image
- Optionally push the Docker image to a registry

## Required Credentials

For the release pipeline to work properly, you need to add credentials to Jenkins:

1. Docker Registry Credentials:
   - ID: `docker-credentials` (or update the `DOCKER_CREDENTIALS_ID` in Jenkinsfile.release)
   - Type: Username with password
   - Enter your Docker registry username and password

## Pod Template Structure

The pod template defined in the Jenkinsfiles includes:

1. **jnlp** container - The standard Jenkins agent container
2. **golang** container - For Go builds and tests
3. **docker** container - For Docker operations
   
The Docker container has the Docker socket mounted from the host, allowing it to create Docker images and push them to registries.

## Customization

You may need to customize the Jenkinsfiles for your environment:

- Update the Docker registry URL in `Jenkinsfile.release` (environment variable `DOCKER_REGISTRY`)
- Adjust the Docker repository name if needed (environment variable `DOCKER_REPOSITORY`)
- Modify the pod template if you need additional tools or different container versions
- Add additional stages or steps as needed for your workflow

## Migrating from GitHub Actions

These Jenkinsfiles were created to replace GitHub Actions workflows. The main differences are:

- Jenkins uses a declarative pipeline syntax instead of YAML
- Docker image building and pushing use different mechanisms
- Credential handling differs between the two systems
- The build environment is a Kubernetes pod with multiple containers

If you find issues during migration, please refer to the Jenkins Pipeline and Kubernetes Plugin documentation. 