[Unit]
Description=Service that runs aws_signing_helper update to keep the AWS credentials refreshed in {{ .SharedCredentialsFilePath }}.

[Service]
User=root
Environment=AWS_SHARED_CREDENTIALS_FILE={{ .SharedCredentialsFilePath }}
ExecStart={{ .SigningHelperBinPath }} update \
        --certificate /etc/iam/pki/server.pem \
        --private-key /etc/iam/pki/server.key \
        --trust-anchor-arn {{ .TrustAnchorARN }} \
        --profile-arn {{ .ProfileARN }} \
        --role-arn {{ .RoleARN }} \
        --role-session-name {{ .NodeName }} \
        --region {{ .Region }}
StandardOutput=journal
StandardError=journal
Restart=always
RestartSec=10
CPUAccounting=true
MemoryAccounting=true

[Install]
WantedBy=multi-user.target
