package main

import (
	"encoding/base64"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func main() {

	// Create EC2 Service to spin up instance
	config := &aws.Config{
		Region: aws.String("us-east-1"),
	}
	svc := ec2.New(session.New(config))

	//Create User data to pass to instance
	data := []byte("#!/bin/bash\nsudo su\nyum update -y\nyum upgrade -y\nyum install httpd -y\nsystemctl start httpd\nsystemctl enable httpd\nchown ec2-user /var/www/*\nchown ec2-user /var/www")
	userData := base64.StdEncoding.EncodeToString(data)
	// Create security group for our instances.
	fmt.Println("Create SecurityGroup Input Structure")
	securityGroupInput := &ec2.CreateSecurityGroupInput{
		DryRun:      aws.Bool(false),
		GroupName:   aws.String("SimpleHTTPService"),
		Description: aws.String("Security groupto allow traffic of the type HTTP!"),
	}

	fmt.Println("Creating Security group")
	securityGroupOutput, err := svc.CreateSecurityGroup(securityGroupInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(*securityGroupOutput.GroupId)
	securityGroupID := securityGroupOutput.GroupId

	// Create Security group ingress rules
	fmt.Println("Adding ingress rules")
	securityGroupIngressInput := &ec2.AuthorizeSecurityGroupIngressInput{
		DryRun:  aws.Bool(false),
		GroupId: securityGroupID,
		IpPermissions: []*ec2.IpPermission{
			{
				FromPort:   aws.Int64(443),
				ToPort:     aws.Int64(443),
				IpProtocol: aws.String("tcp"),
				IpRanges: []*ec2.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("Allow HTTPS"),
					},
				},
			},
			{
				FromPort:   aws.Int64(80),
				ToPort:     aws.Int64(80),
				IpProtocol: aws.String("tcp"),
				IpRanges: []*ec2.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("Allow HTTP"),
					},
				},
			},
			{
				FromPort:   aws.Int64(22),
				ToPort:     aws.Int64(22),
				IpProtocol: aws.String("tcp"),
				IpRanges: []*ec2.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("Allow SSH"),
					},
				},
			},
		},
	}

	securityGroupIngressOutput, err := svc.AuthorizeSecurityGroupIngress(securityGroupIngressInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(securityGroupIngressOutput)

	// Create Launch Template for Instance
	fmt.Println("Creating Launch template.")
	createLaunchTemplateInput := &ec2.CreateLaunchTemplateInput{
		DryRun:             aws.Bool(false),
		LaunchTemplateName: aws.String("OurSimpleServer"),
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			ImageId:      aws.String("ami-0c94855ba95c71c99"),
			InstanceType: aws.String("t2.micro"),
			SecurityGroupIds: []*string{
				securityGroupID,
			},
			KeyName: aws.String("bkeys"),
			TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{

				{
					ResourceType: aws.String("instance"),
					Tags: []*ec2.Tag{

						{
							Key:   aws.String("Name"),
							Value: aws.String("SimpleAwsWebsite"),
						},
					},
				},
			},
			UserData: aws.String(userData),
		},
	}

	createLaunchTemplateOutput, err := svc.CreateLaunchTemplate(createLaunchTemplateInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(*createLaunchTemplateOutput.LaunchTemplate.LaunchTemplateId)
	launchTemplateID := createLaunchTemplateOutput.LaunchTemplate.LaunchTemplateId

	// Create input to spin up an EC2 Instance.
	runInstanceInput := &ec2.RunInstancesInput{
		DryRun:   aws.Bool(false),
		MaxCount: aws.Int64(1),
		MinCount: aws.Int64(1),
		LaunchTemplate: &ec2.LaunchTemplateSpecification{
			LaunchTemplateId: launchTemplateID,
		},
	}

	// Spin up Instance from launch template
	runInstanceOutput, err := svc.RunInstances(runInstanceInput)
	if err != nil {
		log.Fatal(err)
	}

	// Save the instance ID
	instanceID := runInstanceOutput.Instances[0].InstanceId

	// Create Describe instance input to wait until running
	describeInstanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			instanceID,
		},
	}

	// Wait until the instance is running
	fmt.Println("Waiting for instance to be running...")
	waitOutput := svc.WaitUntilInstanceRunning(describeInstanceInput)
	if waitOutput != nil {
		log.Fatal(waitOutput)
	}
	fmt.Println("Instance is running")

	// Get the IP address of the running instance
	describeInstanceOutput, err := svc.DescribeInstances(describeInstanceInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(*describeInstanceOutput.Reservations[0].Instances[0].PublicIpAddress)

}
