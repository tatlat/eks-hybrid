#!/usr/bin/env bash
# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source $REPO_ROOT/hack/common.sh

DATE=date
if [ "$(uname -s)" = "Darwin" ]; then
    DATE=gdate
fi

TEST_CLUSTER_TAG_KEY="Nodeadm-E2E-Tests-Cluster"

CLUSTER_NAME=${1:-}

TEST_CLUSTER_TAG_KEY_FILTER="Name=tag-key,Values=$TEST_CLUSTER_TAG_KEY"
if [ -n "$CLUSTER_NAME" ]; then
    TEST_CLUSTER_TAG_KEY_FILTER+=" Name=tag:$TEST_CLUSTER_TAG_KEY,Values=$CLUSTER_NAME"
fi


function aws() {
    retry build::common::echo_and_run command aws "$@"    
}

function older_than_one_day(){
    created="$1"

    createDate=$($DATE -d"$created" +%s)
    olderThan=$($DATE --date "1 second ago" +%s)
    if [[ $createDate -lt $olderThan ]]; then       
        return 0  # 0 = true
    else       
        return 1  # 1 = false
    fi
}

declare -A CLUSTER_STATUSES=()
function is_eks_cluster_deleted(){
    name="$1"

    # +1 means to return 1 if the key exists, otherwise return nothing
    if [ ! ${CLUSTER_STATUSES[$name]+1} ]; then
        cluster_status="$(command aws eks describe-cluster --name $name --query 'cluster.status' --output text)"
        if [ $? != 0 ]; then
            # 1 = false
            CLUSTER_STATUSES[$name]="DELETING"
        else
            CLUSTER_STATUSES[$name]=$cluster_status
        fi
    fi
    if [ "DELETING" == ${CLUSTER_STATUSES[$name]} ]; then
        # 0 = true
        return 0
    else   
        # 1 = false   
        return 1
    fi
}

function delete_cluster(){
    eks_cluster="$1"
 
     aws eks delete-cluster --name $eks_cluster
} 

function delete_instance(){
    instance_id="$1"
    cluster_name="$2"
   
    aws ec2 terminate-instances --instance-ids $instance_id
}

function delete_peering_connection(){
    peering_connection_id="$1"
    cluster_name="$2"
    
    aws ec2 delete-vpc-peering-connection --vpc-peering-connection-id $peering_connection_id  
}

function delete_role(){
    role="$1"

    if [ ! -z "$(aws iam list-attached-role-policies --role-name $role --query "AttachedPolicies[*]" --output text)" ]; then
        aws iam detach-role-policy --role-name $role --policy-arn "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
    fi
    aws iam delete-role --role-name $role
}

function delete_vpc(){
    vpc="$1"
    cluster_name="$2"

    for internet_gateway in $(aws ec2 describe-internet-gateways --filters "Name=attachment.vpc-id,Values=$vpc" --query "InternetGateways[*].InternetGatewayId"  --output text); do 
        aws ec2 detach-internet-gateway --internet-gateway-id $internet_gateway --vpc-id $vpc
        aws ec2 delete-internet-gateway --internet-gateway-id $internet_gateway
    done

    for subnet in $(aws ec2 describe-subnets --filters "Name=vpc-id,Values=$vpc" --query "Subnets[*].SubnetId" --output text); do 
        aws ec2 delete-subnet --subnet-id $subnet
    done

    main_route_table=$(aws ec2  describe-route-tables --filters "Name=vpc-id,Values=$vpc" "Name=association.main,Values=true" --query "RouteTables[*].RouteTableId" --output text)
    for rt in $(aws ec2 describe-route-tables --filters "Name=vpc-id,Values=$vpc" --query "RouteTables[*].RouteTableId" --output text); do
        if [[ "$rt" != "$main_route_table" ]]; then
            aws ec2 delete-route-table --route-table-id $rt
        fi
    done

    aws ec2 delete-vpc --vpc-id $vpc
}

for eks_cluster in $(aws eks list-clusters --query "clusters" --output text); do
    if [ -n "$CLUSTER_NAME" ] && [ "$eks_cluster" == "$CLUSTER_NAME" ]; then
        delete_cluster $eks_cluster
        break
    fi    
    describe=$(aws eks describe-cluster --name $eks_cluster --query 'cluster.{tags:tags,createdAt:createdAt}')
    if older_than_one_day "$(echo $describe | jq -r ".createdAt")"; then
        if [ "true" == $(echo $describe | jq ".tags | has(\"$TEST_CLUSTER_TAG_KEY\")") ]; then
            delete_cluster $eks_cluster
        fi 
    fi
    
done

# Loop through role names and get tags
for role in $(aws iam list-roles --query 'Roles[*].RoleName' --no-paginate --output text); do
    if [[ $role == *-hybrid-node ]]; then
        cluster_name="$(aws iam list-role-tags --query "Tags[*]" --role-name $role --output json | jq -r "map(select(.Key == \"$TEST_CLUSTER_TAG_KEY\"))[0].Value")"
        if [ -z "$cluster_name" ] || ! is_eks_cluster_deleted $cluster_name; then
            continue
        fi
        if [ -z "$CLUSTER_NAME" ] || [ "$cluster_name" == "$CLUSTER_NAME" ]; then        
            delete_role $role
        fi
    fi
done

for reservations in $(aws ec2 describe-instances --filters $TEST_CLUSTER_TAG_KEY_FILTER --query "Reservations[*].Instances[*].{InstanceId:InstanceId,Tags:Tags,State:State.Name}" --output json | jq -c '.[]'); do
    for ec2 in $(echo $reservations | jq -c '.[]'); do
        if [ "terminated" == "$(echo "$ec2" | jq -r '.State')" ]; then
            continue
        fi
        cluster_name=$(echo $ec2 | jq -r ".Tags | map(select(.Key == \"$TEST_CLUSTER_TAG_KEY\"))[0].Value")
        instance_id=$(echo $ec2 | jq -r ".InstanceId")
        if ! is_eks_cluster_deleted $cluster_name; then
            continue
        fi
        delete_instance $instance_id $cluster_name
    done
done

for peering_connection in $(aws ec2 describe-vpc-peering-connections --filters $TEST_CLUSTER_TAG_KEY_FILTER --query "VpcPeeringConnections[*].{VpcPeeringConnectionId:VpcPeeringConnectionId,Tags:Tags,StatusCode:Status.Code}" --output json | jq -c '.[]'); do     
    if [ "deleted" == "$(echo "$peering_connection" | jq -r '.StatusCode')" ]; then
        continue
    fi
    cluster_name=$(echo "$peering_connection" | jq -r ".Tags | map(select(.Key == \"$TEST_CLUSTER_TAG_KEY\"))[0].Value")
    peering_connection_id=$(echo "$peering_connection" | jq -r ".VpcPeeringConnectionId")
    if ! is_eks_cluster_deleted $cluster_name; then
        continue
    fi
    delete_peering_connection $peering_connection_id $cluster_name
done

for vpc in $(aws ec2 describe-vpcs --filters $TEST_CLUSTER_TAG_KEY_FILTER --query "Vpcs[*].{VpcId:VpcId,Tags:Tags}"| jq -c '.[]'); do
    cluster_name=$(echo $vpc | jq -r ".Tags | map(select(.Key == \"$TEST_CLUSTER_TAG_KEY\"))[0].Value")
    vpc=$(echo $vpc | jq -r ".VpcId")
    if ! is_eks_cluster_deleted $cluster_name; then
        continue
    fi
    delete_vpc $vpc $cluster_name
done



# TODO:
#  hybrid activations
#  registered managed instances
