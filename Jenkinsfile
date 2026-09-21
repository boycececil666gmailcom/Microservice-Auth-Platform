pipeline {
    agent any

    environment {
        CGO_ENABLED = '0'
        GOOS        = 'linux'
        GOARCH      = 'arm64'
    }

    stages {
        stage('Code Quality') {
            steps {
                echo '[Pipeline-Quality] Checking format and static analysis...'
                sh 'test -z "$(gofmt -l cmd internal tests)"'
                sh 'go vet ./...'
            }
        }

        stage('Unit Tests') {
            steps {
                echo '[Pipeline-Test] Running unit tests with race detector...'
                sh 'go test -race -coverprofile=coverage.out ./...'
            }
        }

        stage('Build Lambda Packages') {
            steps {
                echo '[Pipeline-Build] Compiling Shortener Lambda (arm64)...'
                sh '''
                    mkdir -p bin
                    go build -trimpath -ldflags="-s -w" -o bootstrap ./cmd/shortener
                    if command -v zip >/dev/null 2>&1; then
                        zip -q -j bin/shortener.zip bootstrap
                    else
                        python3 -m zipfile -c bin/shortener.zip bootstrap
                    fi
                    rm -f bootstrap
                '''
                echo '[Pipeline-Build] Compiling Analytics Lambda (arm64)...'
                sh '''
                    go build -trimpath -ldflags="-s -w" -o bootstrap ./cmd/analytics
                    if command -v zip >/dev/null 2>&1; then
                        zip -q -j bin/analytics.zip bootstrap
                    else
                        python3 -m zipfile -c bin/analytics.zip bootstrap
                    fi
                    rm -f bootstrap
                '''
            }
        }

        stage('Terraform Validation') {
            steps {
                echo '[Pipeline-Terraform] Validating infrastructure configuration...'
                sh 'terraform fmt -check -recursive infra_tf'
                sh 'terraform -chdir=infra_tf init -backend=false -input=false'
                sh 'terraform -chdir=infra_tf validate'
            }
        }

        stage('Deploy Infrastructure') {
            when {
                anyOf {
                    branch 'master'
                    branch 'main'
                    branch 'GO-AWS-Lambda'
                }
            }
            steps {
                echo '[Pipeline-Deploy] Applying Terraform to AWS...'
                sh 'terraform -chdir=infra_tf init -input=false'
                sh 'terraform -chdir=infra_tf apply -auto-approve -input=false'
            }
        }

        stage('Live E2E Tests') {
            when {
                anyOf {
                    branch 'master'
                    branch 'main'
                    branch 'GO-AWS-Lambda'
                }
            }
            steps {
                echo '[Pipeline-E2E] Executing live E2E tests against API Gateway...'
                sh '''
                    ENDPOINT=$(terraform -chdir=infra_tf output -raw api_endpoint 2>/dev/null | tr -d '\r' | sed 's:/*$::' || true)
                    export GATEWAY_URL="${ENDPOINT:-http://localhost:8000}"
                    go test -tags=e2e -count=1 ./tests/e2e/ -v
                '''
            }
        }
    }

    post {
        always {
            cleanWs()
        }
    }
}
