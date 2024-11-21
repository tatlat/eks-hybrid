import codebuild = require('aws-cdk-lib/aws-codebuild');
import cdk = require('aws-cdk-lib');
import secretsmanager = require('aws-cdk-lib/aws-secretsmanager');
import codepipeline = require('aws-cdk-lib/aws-codepipeline');
import s3 = require('aws-cdk-lib/aws-s3');
import iam = require('aws-cdk-lib/aws-iam');
import codepipeline_actions = require('aws-cdk-lib/aws-codepipeline-actions');
import * as fs from 'fs';
import { kubernetesVersions, cnis, eksHybridBetaBucketARN, eksReleaseManifestHost, builderBaseImage, githubRepo, githubBranch, requiredEnvVars } from './constants';

export class NodeadmBuildStack extends cdk.Stack {
  constructor(scope: cdk.App, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const devStackConfig = JSON.parse(
      fs.readFileSync('cdk_dev_env.json', 'utf-8')
    );

    for (const envVar of requiredEnvVars) {
      if (process.env[envVar] === undefined) {
        throw new Error(`Required environment variable '${envVar}' not set`);
      }
    }

    let goproxy = 'direct';
    if (process.env['GOPROXY'] !== undefined && process.env['GOPROXY'] !== '') {
      goproxy = process.env['GOPROXY']!
    } else {
      console.warn(`GOPROXY env var not set or is empty. Defaulting to '${goproxy}'`);
    }

    const githubTokenSecret = new secretsmanager.Secret(this, 'NodeadmE2ETestsGitHubToken', {
      secretName: 'nodeadm-e2e-tests-github-token',
      description: 'Personal Access Token for authenticating to GitHub',
      secretObjectValue: {
        'github-token': cdk.SecretValue.unsafePlainText(process.env.HYBRID_GITHUB_TOKEN!),
      }
    });

    const goproxySecret = new secretsmanager.Secret(this, 'NodeadmE2ETestsGoproxy', {
      secretName: 'nodeadm-e2e-tests-goproxy',
      description: 'Go module proxy endpoint or mode',
      secretObjectValue: {
        endpoint: cdk.SecretValue.unsafePlainText(goproxy),
      }
    });

    let rhelUsername = ""
    let rhelPassword = ""
    if (process.env['RHEL_USERNAME'] !== undefined && process.env['RHEL_USERNAME'] !== '') {
      rhelUsername = process.env['RHEL_USERNAME']!
    } else {
      console.warn(`'RHEL_USERNAME' env var not set or is empty. This will cause Red Hat credentials secret creation to get skipped, which could cause RHEL tests to fail'`);
    }
    if (process.env['RHEL_PASSWORD'] !== undefined && process.env['RHEL_PASSWORD'] !== '') {
      rhelPassword = process.env['RHEL_PASSWORD']!
    } else {
      console.warn(`'RHEL_PASSWORD' env var not set or is empty. This will cause Red Hat credentials secret creation to get skipped, which could cause RHEL tests to fail'`);
    }

    if (rhelUsername !== '' && rhelUsername !== '') {
      const redhatCredentialsSecret = new secretsmanager.Secret(this, 'NodeadmE2ERedHatCredentials', {
        secretName: 'nodeadm-e2e-tests-redhat-credentials',
        description: 'Username and password for authenticating with Red Hat Subscription Manager ',
        secretObjectValue: {
          'username': cdk.SecretValue.unsafePlainText(rhelUsername),
          'password': cdk.SecretValue.unsafePlainText(rhelPassword),
        },
      });
    } else {
      console.warn(`Red Hat credentials secret creation has been skipped due to empty username and/or password environment variables'`);
    }

    const nodeadmBinaryBucket = new s3.Bucket(this, `nodeadm-binaries-${this.account}`, {
      bucketName: `nodeadm-binaries-${this.account}`,
      enforceSSL: true,
      versioned: true,
      encryption: s3.BucketEncryption.S3_MANAGED,
    });

    const sourceOutput = new codepipeline.Artifact();
    const sourceAction = new codepipeline_actions.GitHubSourceAction({
      actionName: 'GitHubSource',
      owner: devStackConfig.github_username,
      repo: githubRepo,
      branch: githubBranch,
      oauthToken: githubTokenSecret.secretValueFromJson('github-token'),
      output: sourceOutput,
      trigger: codepipeline_actions.GitHubTrigger.NONE,
    });

    const codeBuildProject = new codebuild.PipelineProject(this, 'nodeadm-build', {
      projectName: 'nodeadm-build',
      buildSpec: codebuild.BuildSpec.fromSourceFilename('buildspecs/build-nodeadm.yml'),
      environmentVariables: {
        GOPROXY: {
          type: codebuild.BuildEnvironmentVariableType.SECRETS_MANAGER,
          value: `${goproxySecret.secretArn}:endpoint`,
        },
        MANIFEST_HOST: {
          type: codebuild.BuildEnvironmentVariableType.PLAINTEXT,
          value: eksReleaseManifestHost,
        },
      },
      environment: {
        buildImage: codebuild.LinuxBuildImage.fromDockerRegistry(builderBaseImage),
        computeType: codebuild.ComputeType.LARGE,
        
      },
    });

    const buildOutput = new codepipeline.Artifact();
    const buildAction = new codepipeline_actions.CodeBuildAction({
      actionName: 'Build',
      input: sourceOutput,
      outputs: [buildOutput],
      project: codeBuildProject,
    });

    const integrationTestProject = new codebuild.PipelineProject(this, 'nodeadm-e2e-tests-project', {
      projectName: 'nodeadm-e2e-tests',
      buildSpec: codebuild.BuildSpec.fromSourceFilename('buildspecs/test-nodeadm.yml'),
      environment: {
        buildImage: codebuild.LinuxBuildImage.fromDockerRegistry(builderBaseImage),
        environmentVariables: {
          AWS_REGION: {
            value: this.region,
          },
          ARTIFACTS_BUCKET: {
            type: codebuild.BuildEnvironmentVariableType.PLAINTEXT,
            value: nodeadmBinaryBucket.bucketName,
          },
          GOPROXY: {
            type: codebuild.BuildEnvironmentVariableType.SECRETS_MANAGER,
            value: `${goproxySecret.secretArn}:endpoint`,
          },
        },
      },
    });
    integrationTestProject.role!.attachInlinePolicy(
      new iam.Policy(this, 'nodeadm-e2e-tests-runner-policy', {
        statements: [
          new iam.PolicyStatement({
            actions: [
              'iam:CreateRole',
              'iam:AttachRolePolicy',
              'iam:DeleteRole',
              'iam:DeleteRolePolicy',
              'iam:DetachRolePolicy',
              'iam:GetRole',
              'iam:PassRole',
              'iam:PutRolePolicy',
              'iam:TagRole',
            ],
            resources: [`arn:aws:iam::${this.account}:role/*`],
            effect: iam.Effect.ALLOW,
          }),
          new iam.PolicyStatement({
            actions: [
              'iam:AddRoleToInstanceProfile',
              'iam:CreateInstanceProfile',
              'iam:DeleteInstanceProfile',
              'iam:GetInstanceProfile',
              'iam:RemoveRoleFromInstanceProfile',
            ],
            resources: [`arn:aws:iam::${this.account}:instance-profile/*`],
            effect: iam.Effect.ALLOW,
          }),
          new iam.PolicyStatement({
            actions: [
              'ec2:AcceptVpcPeeringConnection',
              'ec2:AssociateRouteTable',
              'ec2:AttachInternetGateway',
              'ec2:AuthorizeSecurityGroupIngress',
              'ec2:CreateInternetGateway',
              'ec2:CreateRoute',
              'ec2:CreateRouteTable',
              'ec2:CreateSubnet',
              'ec2:CreateTags',
              'ec2:CreateVpc',
              'ec2:CreateVpcPeeringConnection',
              'ec2:DeleteInternetGateway',
              'ec2:DeleteRouteTable',
              'ec2:DeleteSubnet',
              'ec2:DeleteVpc',
              'ec2:DeleteVpcPeeringConnection',
              'ec2:DescribeImages',
              'ec2:DescribeInstances',
              'ec2:DescribeInternetGateways',
              'ec2:DescribeRouteTables',
              'ec2:DescribeSecurityGroups',
              'ec2:DescribeSubnets',
              'ec2:DescribeVpcs',
              'ec2:DetachInternetGateway',
              'ec2:ModifySubnetAttribute',
              'ec2:RebootInstances',
              'ec2:RunInstances',
              'ec2:TerminateInstances',
            ],
            resources: ['*'],
            effect: iam.Effect.ALLOW,
          }),
          new iam.PolicyStatement({
            actions: ['ssm:SendCommand'],
            resources: [
              'arn:aws:ec2:*:*:instance/*',
              'arn:aws:ssm:*:*:managed-instance/*',
              'arn:aws:ssm:*::document/AWS-RunShellScript',
            ],
            effect: iam.Effect.ALLOW,
          }),
          new iam.PolicyStatement({
            actions: ['ssm:GetCommandInvocation'],
            resources: [`arn:aws:ssm:*:${this.account}:*`],
            effect: iam.Effect.ALLOW,
          }),
          new iam.PolicyStatement({
            actions: ['ssm:CreateActivation'],
            resources: ['*'],
            effect: iam.Effect.ALLOW,
          }),
          new iam.PolicyStatement({
            actions: ['ssm:GetParameter'],
            resources: [
              `arn:aws:ssm:${this.region}:${this.account}:parameter/*`,
              `arn:aws:ssm:${this.region}::parameter/*`,
            ],
            effect: iam.Effect.ALLOW,
          }),
          new iam.PolicyStatement({
            actions: ['secretsmanager:GetSecretValue'],
            resources: [`arn:aws:secretsmanager:${this.region}:${this.account}:secret:*`],
          }),
          new iam.PolicyStatement({
            effect: iam.Effect.ALLOW,
            actions: ['s3:GetObject', 's3:HeadObject', 's3:ListBucket'],
            resources: [eksHybridBetaBucketARN, `${eksHybridBetaBucketARN}/*`],
          }),
          new iam.PolicyStatement({
            effect: iam.Effect.ALLOW,
            actions: [
              'eks:CreateAccessEntry',
              'eks:CreateCluster',
              'eks:DescribeCluster',
              'eks:DeleteCluster',
              'eks:TagResource',
            ],
            resources: [`arn:aws:eks:${this.region}:${this.account}:cluster/*`],
          }),
          new iam.PolicyStatement({
            effect: iam.Effect.ALLOW,
            actions: ['eks:DeleteAccessEntry', 'eks:DescribeAccessEntry', 'eks:ListAssociatedAccessPolicies'],
            resources: [`arn:aws:eks:${this.region}:${this.account}:access-entry/*`],
          }),
          new iam.PolicyStatement({
            effect: iam.Effect.ALLOW,
            actions: [
              'cloudformation:DescribeStacks',
              'cloudformation:CreateStack',
              'cloudformation:UpdateStack',
              'cloudformation:DeleteStack',
            ],
            resources: [`arn:aws:cloudformation:${this.region}:${this.account}:stack/*`],
          }),
        ],
      }),
    );

    const e2eTestsActions: Array<codepipeline_actions.CodeBuildAction> = [];
    for (const kubeVersion of kubernetesVersions) {
      for (const cni of cnis) {
        const actionName = `kube-${kubeVersion.replace('.', '-')}-${cni}`;

        const e2eTestsAction = new codepipeline_actions.CodeBuildAction({
          actionName: actionName,
          input: buildOutput,
          project: integrationTestProject,
          environmentVariables: {
            KUBERNETES_VERSION: {
              value: kubeVersion,
            },
            CNI: {
              value: cni,
            },
          },
        });
        e2eTestsActions.push(e2eTestsAction);
      }
    }

    const codeBuildReleaseProject = new codebuild.PipelineProject(this, 'nodeadm-upload-artifacts', {
      projectName: 'nodeadm-upload-artifacts',
      buildSpec: codebuild.BuildSpec.fromSourceFilename('buildspecs/dev-release-nodeadm.yml'),
      environmentVariables: {
        ARTIFACTS_BUCKET: {
          type: codebuild.BuildEnvironmentVariableType.PLAINTEXT,
          value: nodeadmBinaryBucket.bucketName,
        },
      },
      environment: {
        buildImage: codebuild.LinuxBuildImage.fromDockerRegistry(builderBaseImage),
      },
    });
    codeBuildReleaseProject.role!.addToPrincipalPolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: ['s3:PutObject*'],
        resources: [nodeadmBinaryBucket.bucketArn, `${nodeadmBinaryBucket.bucketArn}/*`],
      }),
    );
    const devReleaseAction = new codepipeline_actions.CodeBuildAction({
      actionName: 'Upload-Artifacts',
      input: buildOutput,
      project: codeBuildReleaseProject,
    });

    // Create the CodePipeline with the private GitHub source
    const pipeline = new codepipeline.Pipeline(this, 'nodeadm-e2e-tests-pipeline', {
      pipelineName: 'nodeadm-e2e-tests',
      pipelineType: codepipeline.PipelineType.V2,
      restartExecutionOnUpdate: false,
      stages: [
        {
          stageName: 'Source',
          actions: [sourceAction],
        },
        {
          stageName: 'Build',
          actions: [buildAction],
        },
        {
          stageName: 'Upload-Artifacts',
          actions: [devReleaseAction],
        },
        {
          stageName: 'E2E-Tests',
          actions: [...e2eTestsActions],
        },
      ],
    });
  }
}
