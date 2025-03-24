[profile %v]
region = {{ .Region }}
credential_process = {{ .SigningHelperBinPath }} credential-process --certificate {{ .CertificatePath }} --private-key {{ .PrivateKeyPath }} --trust-anchor-arn {{ .TrustAnchorARN }} --profile-arn {{ .ProfileARN }} --role-arn {{ .RoleARN }} --role-session-name {{ .NodeName }}{{ if .ProxyEnabled }} --with-proxy{{end}}
