[Unit]
Description=Service that runs aws_signing_helper update to keep the AWS credentials refreshed in {{ .SharedCredentialsFilePath }}.

[Service]
User=root
Environment=AWS_SHARED_CREDENTIALS_FILE={{ .SharedCredentialsFilePath }}
ExecStart={{ .SigningHelperBinPath }} update \
        --certificate {{ .CertificatePath }} \
        --private-key {{ .PrivateKeyPath }} \
        --trust-anchor-arn {{ .TrustAnchorARN }} \
        --profile-arn {{ .ProfileARN }} \
        --role-arn {{ .RoleARN }} \
        --role-session-name {{ .NodeName }} \
        --region {{ .Region }}{{ if .ProxyEnabled }} --with-proxy{{end}}
StandardOutput=journal
StandardError=journal
Restart=always
RestartSec=10
CPUAccounting=true
MemoryAccounting=true

[Install]
WantedBy=multi-user.target
