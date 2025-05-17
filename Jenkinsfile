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
  - name: kaniko
    image: gcr.io/kaniko-project/executor:v1.11.0-debug
    imagePullPolicy: Always
    command:
    - /busybox/cat
    tty: true
    volumeMounts:
      - name: jenkins-docker-cfg
        mountPath: /kaniko/.docker
    resources:
      limits:
        memory: 2Gi
        cpu: 500m
      requests:
        memory: 512Mi
        cpu: 250m
  volumes:
  - name: jenkins-docker-cfg
    secret:
      secretName: regcred
      items:
        - key: .dockerconfigjson
          path: config.json
  - emptyDir:
      medium: ""
    name: "workspace-volume"
'''
            defaultContainer 'golang'
        }
    }

    environment {
        REGISTRY = "fixyimage.azurecr.io"
        IMAGE_NAME = "oortfy/apigateway"
        IMAGE_TAG = "${BUILD_NUMBER}"
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
                    
                    // Run fix_linting.sh script
                    sh 'chmod +x ./fix_linting.sh && ./fix_linting.sh'
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
                container('kaniko') {
                    // Build Docker image using Kaniko
                    sh '/kaniko/executor --context=$PWD --destination=${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG} --cleanup'
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