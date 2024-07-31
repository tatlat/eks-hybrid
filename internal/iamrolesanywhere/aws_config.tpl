[profile %v]
region = {{ .Region }}
credential_process = {{ .SigningHelperBinPath }} credential-process --certificate /etc/iam/pki/server.pem --private-key /etc/iam/pki/server.key --trust-anchor-arn {{ .TrustAnchorARN }} --profile-arn {{ .ProfileARN }} --role-arn {{ .RoleARN }}
