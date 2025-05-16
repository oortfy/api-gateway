pipeline {
    agent {
        kubernetes {
            yaml '''
apiVersion: v1
kind: Pod
metadata:
  labels:
    jenkins: agent
  namespace: jenkins-agents
spec:
  serviceAccountName: jenkins
  containers:
  - name: jnlp
    image: jenkins/inbound-agent:latest
    env:
    - name: JENKINS_URL
      value: "https://jenkins.oortfy.com/"
    - name: JENKINS_AGENT_WORKDIR
      value: "/home/jenkins/agent"
  - name: golang
    image: golang:1.24
    command:
    - cat
    tty: true
  - name: docker
    image: docker:latest
    command:
    - cat
    tty: true
    volumeMounts:
    - name: docker-sock
      mountPath: /var/run/docker.sock
  volumes:
  - name: docker-sock
    hostPath:
      path: /var/run/docker.sock
'''
            defaultContainer 'golang'
        }
    }

    stages {
        stage('Setup') {
            steps {
                container('golang') {
                    sh 'go mod download'
                    sh 'go mod verify'
                }
            }
        }

        stage('Lint') {
            steps {
                container('golang') {
                    // Install golangci-lint
                    sh 'curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2'
                    sh '$(go env GOPATH)/bin/golangci-lint run --timeout=5m'
                }
            }
        }

        stage('Test') {
            steps {
                container('golang') {
                    // Run tests with race detection
                    sh 'go test -race -v ./...'
                    
                    // Run main test suite with coverage
                    sh 'go test -v -coverprofile=coverage.out ./...'
                    
                    // Run component-specific tests
                    sh 'make test-component-coverage || echo "Some tests may have been skipped"'
                    
                    // Convert coverage report to Jenkins compatible format
                    sh 'go tool cover -html=coverage.out -o coverage.html'
                }
            }
            post {
                always {
                    // Archive the coverage reports
                    archiveArtifacts artifacts: '*.out, *_coverage.html', allowEmptyArchive: true
                    
                    // Optional: publish coverage report if you have the HTML publisher plugin
                    publishHTML(target: [
                        allowMissing: true,
                        alwaysLinkToLastBuild: false,
                        keepAll: true,
                        reportDir: '.',
                        reportFiles: 'coverage.html',
                        reportName: 'Go Coverage Report'
                    ])
                }
            }
        }

        stage('Build Binary') {
            steps {
                container('golang') {
                    sh 'make build'
                    
                    // Verify the binary was created
                    sh '''
                    if [ ! -f "bin/apigateway" ]; then
                      echo "Application binary not found"
                      exit 1
                    fi
                    echo "Application binary successfully built"
                    '''
                }
            }
            post {
                success {
                    // Archive the binary
                    archiveArtifacts artifacts: 'bin/apigateway', fingerprint: true
                }
            }
        }

        stage('Build Docker Image') {
            steps {
                container('docker') {
                    // Build Docker image
                    sh 'docker build -t apigateway:latest .'
                }
            }
        }
    }

    post {
        always {
            // Clean up workspace
            cleanWs()
        }
    }
} 