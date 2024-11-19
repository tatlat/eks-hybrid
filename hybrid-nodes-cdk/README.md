# Hybrid Nodes CDK Project

This CDK project deploys the AWS resources required to build and test nodeadm in your own account using the code in this repo.

## Deployment instructions

1. Run the `gen-cdk-env` Bash script and follow the prompts to create the config file with your AWS Account ID and region that you wish to deploy the stack to.

2. Export the following environment variables on your terminal:
    * `HYBRID_GITHUB_TOKEN`: This represents the GitHub Access Token required to authenticate CodePipelines with GitHub. This token needs to have `repo` scope. Refer to the [CodePipelines GitHub Source Action documentation](https://docs.aws.amazon.com/cdk/api/v2/docs/aws-cdk-lib.aws_codepipeline_actions-readme.html#github) to learn more. **If you do not set this environment variable, the CDK synthesis and deployment will fail.**
    * `GOPROXY`: This can be set to `direct` or `off` mode, or a public Go proxy endpoint, such as `proxy.golang.org` or a deployment of [Athens Go Module proxy](https://docs.gomods.io). **If you do not set this environment variable, the default value of `direct` will be used.**
    * `RHEL_USERNAME`: This is the username to be used to authenticate with Red Hat Subscription Manager to consume packages for RHEL OS. **If you do not set this environment variable, the creation of the Red Hat credentials secret will be skipped which could cause RHEL tests to fail**
    * `RHEL_PASSWORD`: This is the password to be used to authenticate with Red Hat Subscription Manager to consume packages for RHEL OS. **If you do not set this environment variable, the creation of the Red Hat credentials secret will be skipped which could cause RHEL tests to fail**

3. Run `npm install` to install all the required dependencies.

4. Run `cdk bootstrap` to bootstrap your AWS environment (account and region) where the stack is to be deployed.

5. **[Optional]** Run `cdk synth` to ensure CloudFormation stack synthesis is successful.

6. Run `cdk deploy` to deploy the Hybrid nodes stack to your account. This will cause the different AWS resources such as S3 Bucket, CodeBuild Project, CodePipelines and Secrets to get created.

**_NOTE:_** The E2E test pipeline will be triggered the first time it is created in your account, as this is a default behavior of CodePipelines. However subsequent commits on the `main` branch of the configured GitHub repo will _NOT_ automatically trigger the pipeline, and so you will need to manually trigger it.
