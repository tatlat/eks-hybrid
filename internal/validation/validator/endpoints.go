package validator

import (
	"fmt"
	"time"

	"github.com/aws/eks-hybrid/internal/validation/logger"
	"github.com/aws/eks-hybrid/internal/validation/util"
)

var endpoints = []string{
	"rolesanywhere.amazonaws.com",
}

var regionEndpoints = []string{
	"eks.%s.amazonaws.com",
	"fips.eks.%s.amazonaws.com",
	"amazon-ssm.s3.%s.amazonaws.com",
	"ecr-public.%s.amazonaws.com",
	"s3.%s.amazonaws.com",
	"rolesanywhere.%s.amazonaws.com",
	"ssm.%s.amazonaws.com",
}

const (
	protocal = "tcp"
	port     = ":443"
)

type Endpoints struct {
	regionCode string
	netClient  util.NetClient
}

func NewEndpoints(r string, n util.NetClient) *Endpoints {
	return &Endpoints{regionCode: r, netClient: n}
}

func DefaultEndpoints(region string) *Endpoints {
	return NewEndpoints(region, &util.DefaultNetClient{})
}

func (v *Endpoints) Validate() error {
	var errs []error
	for _, endpoint := range endpoints {
		err := v.ValidateEndpoints(protocal, endpoint, port)
		if err != nil {
			logger.MarkFail(err.Error())
			errs = append(errs, err)
		} else {
			logger.MarkPass("request successful: " + endpoint)
		}
	}

	// regional
	for _, endpoint := range regionEndpoints {
		endpoint = fmt.Sprintf(endpoint, v.regionCode)
		err := v.ValidateEndpoints(protocal, endpoint, port)
		if err != nil {
			logger.MarkFail(err.Error())
			errs = append(errs, err)
		} else {
			logger.MarkPass("request successful: " + endpoint)
		}
	}

	if len(errs) > 0 {
		return &FailError{"endpoints validation failed"}
	}
	return nil
}

func (v *Endpoints) ValidateEndpoints(protocal string, host string, port string) error {
	timeout := 5 * time.Second
	response, err := v.netClient.DialTimeout(protocal, host+port, timeout)
	if err != nil {
		return fmt.Errorf("endpoint not found: %v ", err)
	}
	defer response.Close()

	return nil
}
