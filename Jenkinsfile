pipeline {
    agent any

    parameters {
        booleanParam(name: 'PLAN_INFRA', defaultValue: false, description: 'Create an AWS Terraform plan')
        booleanParam(name: 'DEPLOY', defaultValue: false, description: 'Apply the reviewed Terraform plan')
        choice(name: 'ENVIRONMENT', choices: ['dev', 'staging', 'prod'], description: 'Target environment')
    }

    environment {
        CGO_ENABLED = '1'
        TF_IN_AUTOMATION = 'true'
    }

    stages {
        stage('Code Quality') {
            steps {
                sh 'test -z "$(gofmt -l cmd internal tests)"'
                sh 'go vet ./...'
            }
        }

        stage('Unit Tests') {
            steps {
                sh 'go test -race -coverprofile=coverage.out ./...'
            }
        }

        stage('Build Lambda Packages') {
            steps {
                sh '''
                    mkdir -p bin
                    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bootstrap ./cmd/shortener
                    zip -q -j bin/shortener.zip bootstrap
                    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bootstrap ./cmd/analytics
                    zip -q -j bin/analytics.zip bootstrap
                    rm -f bootstrap
                '''
            }
        }

        stage('Terraform Validation') {
            steps {
                sh 'terraform fmt -check -recursive infra_tf'
                sh 'terraform -chdir=infra_tf init -backend=false -reconfigure -input=false'
                sh 'terraform -chdir=infra_tf validate'
            }
        }

        stage('Terraform Plan') {
            when {
                expression { params.PLAN_INFRA || params.DEPLOY }
            }
            steps {
                sh '''
                    test -f /workspace/infra_tf/backend.hcl
                    cp /workspace/infra_tf/backend.hcl infra_tf/backend.hcl
                    if [ -f /workspace/infra_tf/terraform.tfvars ]; then
                        cp /workspace/infra_tf/terraform.tfvars infra_tf/terraform.tfvars
                    fi
                    terraform -chdir=infra_tf init -reconfigure -input=false -backend-config=backend.hcl
                    terraform -chdir=infra_tf plan -input=false -var="environment=${ENVIRONMENT}" -out=tfplan
                '''
            }
        }

        stage('Deployment Approval') {
            when {
                expression { params.DEPLOY }
            }
            steps {
                input message: "Apply the reviewed ${params.ENVIRONMENT} plan?", ok: 'Deploy'
            }
        }

        stage('Deploy Infrastructure') {
            when {
                expression { params.DEPLOY }
            }
            steps {
                sh 'terraform -chdir=infra_tf apply -input=false tfplan'
            }
        }

        stage('Live E2E Tests') {
            when {
                expression { params.DEPLOY }
            }
            steps {
                sh '''
                    ENDPOINT=$(terraform -chdir=infra_tf output -raw api_endpoint | tr -d '\r' | sed 's:/*$::')
                    GATEWAY_URL="$ENDPOINT" go test -tags=e2e -count=1 ./tests/e2e/ -v
                '''
            }
        }
    }

    post {
        always {
            archiveArtifacts artifacts: 'coverage.out,bin/*.zip', allowEmptyArchive: true, fingerprint: true
        }
    }
}
