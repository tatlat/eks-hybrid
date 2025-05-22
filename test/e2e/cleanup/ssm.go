package cleanup

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/go-logr/logr"
	"slices"

	"github.com/aws/eks-hybrid/test/e2e/constants"
	"github.com/aws/eks-hybrid/test/e2e/errors"
)

const managedInstanceResourceType = "ManagedInstance"

type SSMCleaner struct {
	ssm    *ssm.Client
	logger logr.Logger
}

func NewSSMCleaner(ssm *ssm.Client, logger logr.Logger) *SSMCleaner {
	return &SSMCleaner{
		ssm:    ssm,
		logger: logger,
	}
}

func (s *SSMCleaner) ListActivationsForNode(ctx context.Context, nodeName, clusterName string) ([]string, error) {
	filterInput := FilterInput{ClusterName: clusterName}
	return s.listActivations(ctx, func(activation *types.Activation) bool {
		return shouldDeleteActivation(activation, filterInput) && aws.ToString(activation.DefaultInstanceName) == nodeName
	})
}

func (s *SSMCleaner) ListActivations(ctx context.Context, filterInput FilterInput) ([]string, error) {
	return s.listActivations(ctx, func(activation *types.Activation) bool {
		return shouldDeleteActivation(activation, filterInput)
	})
}

func (s *SSMCleaner) listActivations(ctx context.Context, shouldDelete func(*types.Activation) bool) ([]string, error) {
	paginator := ssm.NewDescribeActivationsPaginator(s.ssm, &ssm.DescribeActivationsInput{})

	var activationIDs []string
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing SSM activations: %w", err)
		}

		for _, activation := range output.ActivationList {
			if shouldDelete(&activation) {
				activationIDs = append(activationIDs, *activation.ActivationId)
			}
		}
	}

	return activationIDs, nil
}

func (s *SSMCleaner) DeleteActivation(ctx context.Context, activationID string) error {
	_, err := s.ssm.DeleteActivation(ctx, &ssm.DeleteActivationInput{
		ActivationId: aws.String(activationID),
	})
	if err != nil && errors.IsType(err, &types.InvalidActivation{}) {
		s.logger.Info("SSM activation already deleted", "activationId", activationID)
		return nil
	}

	if err != nil {
		return fmt.Errorf("deleting SSM activation: %w", err)
	}
	s.logger.Info("Deleted activation", "activationId", activationID)
	return nil
}

func (s *SSMCleaner) ListManagedInstancesByActivationID(ctx context.Context, activationIDs ...string) ([]string, error) {
	input := &ssm.DescribeInstanceInformationInput{
		Filters: []types.InstanceInformationStringFilter{
			{
				Key:    aws.String("ActivationIds"),
				Values: activationIDs,
			},
		},
	}

	return s.listManagedInstances(ctx, input, func(instance *types.InstanceInformation, tags []types.Tag) bool {
		return slices.Contains(activationIDs, *instance.ActivationId)
	})
}

func (s *SSMCleaner) ListManagedInstances(ctx context.Context, filterInput FilterInput) ([]string, error) {
	// These filters are mostly just to limit the number of resources returned
	// the source of truth for filtering is done in shouldDeleteManagedInstance
	input := &ssm.DescribeInstanceInformationInput{}
	if filterInput.ClusterName != "" {
		input.Filters = []types.InstanceInformationStringFilter{
			{
				Key:    aws.String("tag:" + constants.TestClusterTagKey),
				Values: []string{filterInput.ClusterName},
			},
		}
	} else {
		input.Filters = []types.InstanceInformationStringFilter{
			{
				Key:    aws.String("tag-key"),
				Values: []string{constants.TestClusterTagKey},
			},
		}
	}

	return s.listManagedInstances(ctx, input, func(instance *types.InstanceInformation, tags []types.Tag) bool {
		return shouldDeleteManagedInstance(instance, tags, filterInput)
	})
}

func (s *SSMCleaner) listManagedInstances(ctx context.Context, input *ssm.DescribeInstanceInformationInput, shouldDelete func(*types.InstanceInformation, []types.Tag) bool) ([]string, error) {
	var instanceIDs []string

	paginator := ssm.NewDescribeInstanceInformationPaginator(s.ssm, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing SSM managed instances: %w", err)
		}
		for _, instance := range output.InstanceInformationList {
			if instance.ResourceType != managedInstanceResourceType {
				continue
			}
			output, err := s.ssm.ListTagsForResource(ctx, &ssm.ListTagsForResourceInput{
				ResourceId:   aws.String(*instance.InstanceId),
				ResourceType: types.ResourceTypeForTaggingManagedInstance,
			})
			if err != nil && errors.IsType(err, &types.InvalidResourceId{}) {
				// skipping log since we are possiblying checking managed instances we do not
				// intend to delete
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("getting tags for managed instance %s: %w", *instance.InstanceId, err)
			}

			if shouldDelete(&instance, output.TagList) {
				instanceIDs = append(instanceIDs, *instance.InstanceId)
			}
		}
	}

	return instanceIDs, nil
}

func (s *SSMCleaner) DeleteManagedInstance(ctx context.Context, instanceID string) error {
	_, err := s.ssm.DeregisterManagedInstance(ctx, &ssm.DeregisterManagedInstanceInput{
		InstanceId: aws.String(instanceID),
	})
	if err != nil && errors.IsType(err, &types.InvalidInstanceId{}) {
		s.logger.Info("Managed instance already deregistered", "instanceId", instanceID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("deregistering managed instance: %w", err)
	}

	s.logger.Info("Deregistered managed instance", "instanceId", instanceID)
	return nil
}

func (s *SSMCleaner) ListParameters(ctx context.Context, filterInput FilterInput) ([]string, error) {
	paginator := ssm.NewDescribeParametersPaginator(s.ssm, &ssm.DescribeParametersInput{})

	var parameterNames []string
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing SSM parameters: %w", err)
		}

		for _, parameter := range output.Parameters {
			tags, err := s.ssm.ListTagsForResource(ctx, &ssm.ListTagsForResourceInput{
				ResourceId:   aws.String(*parameter.Name),
				ResourceType: types.ResourceTypeForTaggingParameter,
			})

			if err != nil && errors.IsType(err, &types.InvalidResourceId{}) {
				// skipping log since we are possiblying checking parameters we do not
				// intend to delete
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("getting tags for SSM parameter %s: %w", *parameter.Name, err)
			}

			if shouldDeleteParameter(parameter, tags.TagList, filterInput) {
				parameterNames = append(parameterNames, *parameter.Name)
			}
		}
	}

	return parameterNames, nil
}

func (s *SSMCleaner) DeleteParameter(ctx context.Context, parameterName string) error {
	_, err := s.ssm.DeleteParameter(ctx, &ssm.DeleteParameterInput{
		Name: aws.String(parameterName),
	})
	if err != nil && errors.IsType(err, &types.ParameterNotFound{}) {
		s.logger.Info("SSM parameter already deleted", "parameterName", parameterName)
		return nil
	}
	if err != nil {
		return fmt.Errorf("deleting SSM parameter %s: %w", parameterName, err)
	}
	s.logger.Info("Deleted SSM parameter", "parameterName", parameterName)
	return nil
}

func shouldDeleteManagedInstance(instance *types.InstanceInformation, tags []types.Tag, input FilterInput) bool {
	var customTags []Tag
	for _, tag := range tags {
		customTags = append(customTags, Tag{
			Key:   *tag.Key,
			Value: *tag.Value,
		})
	}

	resource := ResourceWithTags{
		ID:           *instance.InstanceId,
		CreationTime: aws.ToTime(instance.RegistrationDate),
		Tags:         customTags,
	}

	return shouldDeleteResource(resource, input)
}

func shouldDeleteActivation(activation *types.Activation, input FilterInput) bool {
	var tags []Tag
	for _, tag := range activation.Tags {
		tags = append(tags, Tag{
			Key:   *tag.Key,
			Value: *tag.Value,
		})
	}

	resource := ResourceWithTags{
		ID:           *activation.ActivationId,
		CreationTime: aws.ToTime(activation.CreatedDate),
		Tags:         tags,
	}
	return shouldDeleteResource(resource, input)
}

func shouldDeleteParameter(parameter types.ParameterMetadata, tags []types.Tag, input FilterInput) bool {
	customTags := []Tag{}
	for _, tag := range tags {
		customTags = append(customTags, Tag{
			Key:   *tag.Key,
			Value: *tag.Value,
		})
	}

	resource := ResourceWithTags{
		ID:           *parameter.Name,
		CreationTime: aws.ToTime(parameter.LastModifiedDate),
		Tags:         customTags,
	}

	return shouldDeleteResource(resource, input)
}
