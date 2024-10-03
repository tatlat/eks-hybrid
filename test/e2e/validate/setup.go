package validate

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const (
	amdBinaryObjectKey = "amd64/nodeadm"
	armBinaryObjectKey = "arm64/nodeadm"
	configObjectKey    = "nodeconfig.yaml"
	changeBinaryMode   = "chmod +x nodeadm"
	runBinary          = "./nodeadm validate --config-source file://./nodeconfig.yaml"

	s3BucketName = "ARTIFACTS_BUCKET"
)

// global retrier
var TestRetrier = Retrier{
	MaxRetries:    100,
	InitialDelay:  1 * time.Second,
	MaxDelay:      1 * time.Minute,
	BackoffFactor: 1.0,
}

type EC2Instance struct {
	nodeadmE2ERoleEnv string
	imageID           string
	ec2Type           string
	volumeSize        int64
	arch              string
	userData          string
}

func SetUpTest(region string, instance EC2Instance, testName string) (*TestRunner, error) {
	// create new command runner
	runner := NewTestRunner()
	fmt.Println("Test: ", testName)

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		fmt.Println("failed to create session: ", err)
		return runner, err
	}

	// CreateEC2 returns EC2 service client, intance id, err
	svcEC2, id, err := CreateEC2(sess, instance)
	if err != nil {
		fmt.Println("failed to create EC2 instance: ", err)
		return runner, err
	}
	runner.svcEC2 = svcEC2
	runner.InstanceID = id

	// Create ssm to send command
	svcSSM, err := CreateSSM(sess, runner.InstanceID)
	if err != nil {
		fmt.Println("failed to create SSM client: ", err)
		return runner, err
	}
	runner.svcSSM = svcSSM

	err = RegisterCommands(sess, runner, instance)
	if err != nil {
		fmt.Println("failed to get s3 presigned URLs: ", err)
		return runner, err
	}

	return runner, nil
}

func CreateEC2(sess *session.Session, instance EC2Instance) (*ec2.EC2, string, error) {

	// Create EC2 service client
	svc := ec2.New(sess)

	userDataEncoded := base64.StdEncoding.EncodeToString([]byte(instance.userData))

	// Specify the details of the instance
	runResult, err := svc.RunInstances(&ec2.RunInstancesInput{
		ImageId:      aws.String(instance.imageID),
		InstanceType: aws.String(instance.ec2Type),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: aws.String(instance.nodeadmE2ERoleEnv),
		},
		BlockDeviceMappings: []*ec2.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &ec2.EbsBlockDevice{
					VolumeSize: aws.Int64(instance.volumeSize),
				},
			},
		},
		UserData: &userDataEncoded,
		MetadataOptions: &ec2.InstanceMetadataOptionsRequest{HttpTokens: aws.String("required"), HttpPutResponseHopLimit: aws.Int64(int64(2))},
	})

	if err != nil {
		fmt.Println("Could not create instance", err)
		return nil, "", err
	}

	fmt.Println("Created instance: ", *runResult.Instances[0].InstanceId)

	return svc, *runResult.Instances[0].InstanceId, nil
}

func WaitForEC2Ready(svc *ssm.SSM, instanceID string) error {

	// Define the test SSM command input
	sendCommandInputTest := &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds:  aws.StringSlice([]string{instanceID}),
		Parameters: map[string][]*string{
			"commands": {
				aws.String("ls"), // Test Command to run
			},
		},
	}

	// Wait for the instance to ready
	err := TestRetrier.Retry(func() error {
		// Send the test SSM command
		output, err := svc.SendCommand(sendCommandInputTest)
		if err != nil {
			fmt.Println("Failed to send SSM command:", err)
			return err
		}
		fmt.Println("Success to send SSM command:", *output.Command.Parameters["commands"][0])
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func CreateSSM(sess *session.Session, instanceID string) (*ssm.SSM, error) {
	// Create an SSM service client
	svc := ssm.New(sess)

	err := WaitForEC2Ready(svc, instanceID)
	if err != nil {
		fmt.Println("Failed waiting for ssm to be ready:", err)
		return nil, err
	}

	return svc, nil
}

func RegisterCommands(sess *session.Session, runner *TestRunner, instance EC2Instance) error {
	// Create an S3 service client
	s3Svc := s3.New(sess)

	// Get the S3 bucket name from the environment variable
	s3BucketName := os.Getenv(s3BucketName)
	if s3BucketName == "" {
		return fmt.Errorf("%s environment variable is not set", s3BucketName)
	}

	binaryKeys := []string{configObjectKey}
	if instance.arch == "amd64" {
		binaryKeys = append(binaryKeys, amdBinaryObjectKey)
	} else {
		binaryKeys = append(binaryKeys, armBinaryObjectKey)
	}

	// Expiration time for the pre-signed URLs (12 hours)
	expirationTime := time.Duration(12 * time.Hour)

	// Generate pre-signed URLs for each object key
	for _, binaryKey := range binaryKeys {
		fmt.Printf("Attempting to get presign URL for object '%s' from the bucket '%s'", binaryKey, s3BucketName)
		req, _ := s3Svc.GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(s3BucketName),
			Key:    aws.String(binaryKey),
		})
		presignedURL, err := req.Presign(expirationTime)
		if err != nil {
			return fmt.Errorf("error generating pre-signed URL for %s %v", binaryKey, err)
		}

		fmt.Printf("Presign URL: %s", presignedURL)

		// get the binary name
		parts := strings.Split(binaryKey, "/")
		binary := parts[len(parts)-1]

		fmt.Println("Binary: ", binary)
		runner.RegisterCommands(
			fmt.Sprintf("curl -L \"%s\" -o %s", presignedURL, binary),
		)
	}

	// register the cmd for running tests
	runner.RegisterCommands(
		changeBinaryMode,
		runBinary,
	)
	return nil
}
