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

	//Create Key pair
	createKeyPairInput := &ec2.CreateKeyPairInput{
		KeyName: aws.String("bkeys"),
	}
	fmt.Println("Creating key pair")
	createKeyPairOutput, err := svc.CreateKeyPair(createKeyPairInput)
	if err != nil {
		log.Fatal(err)
	}
	err = svc.WaitUntilKeyPairExists(&ec2.DescribeKeyPairsInput{
		KeyPairIds: []*string{
			aws.String(*createKeyPairOutput.KeyPairId),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Key-Pair has been created!")

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

	instancePublicIPAddress := *describeInstanceOutput.Reservations[0].Instances[0].PublicIpAddress
	instanceVolumeID := aws.String(*describeInstanceOutput.Reservations[0].Instances[0].BlockDeviceMappings[0].Ebs.VolumeId)
	instanceSubnet := aws.String(*describeInstanceOutput.Reservations[0].Instances[0].SubnetId)
	fmt.Println(instancePublicIPAddress)
	fmt.Println(instanceVolumeID)

	// Get the Availability Zone to create snapshot later
	describeSubnetInput := &ec2.DescribeSubnetsInput{
		SubnetIds: []*string{
			instanceSubnet,
		},
	}

	describeSubnetOutput, err := svc.DescribeSubnets(describeSubnetInput)
	if err != nil {
		log.Fatal(err)
	}

	// Save the Availability Zone
	instanceAZ := aws.String(*describeSubnetOutput.Subnets[0].AvailabilityZone)

	// Create snapshot input to backup volume
	createSnapShotInput := &ec2.CreateSnapshotInput{
		Description: aws.String("HTTP Server Snapshot"),
		VolumeId:    instanceVolumeID,
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("snapshot"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("SimpleHttpServerSnapShot"),
					},
				},
			},
		},
	}

	// Create snapshot
	createSnapShotOutput, err := svc.CreateSnapshot(createSnapShotInput)
	if err != nil {
		log.Fatal(err)
	}
	snapShotID := aws.String(*createSnapShotOutput.SnapshotId)

	// Wait for Snapshot to be completed creating
	describeSnapshotsInput := &ec2.DescribeSnapshotsInput{
		SnapshotIds: []*string{
			snapShotID,
		},
	}

	fmt.Println("Creating Snapshot...")
	waitForSnapshotComplete := svc.WaitUntilSnapshotCompleted(describeSnapshotsInput)
	if waitForSnapshotComplete != nil {
		log.Fatal(waitForSnapshotComplete)
	}
	fmt.Println("Snapshot has been created.")

	// Create new volume from snapshot
	createVolumeInput := &ec2.CreateVolumeInput{
		AvailabilityZone: instanceAZ,
		SnapshotId:       snapShotID,
	}

	createVolumeOutput, err := svc.CreateVolume(createVolumeInput)
	if err != nil {
		log.Fatal(err)
	}

	// Save volument ID and wait until volume is available
	snapShotVolumeID := aws.String(*createVolumeOutput.VolumeId)
	describeVolumeInput := &ec2.DescribeVolumesInput{
		VolumeIds: []*string{
			snapShotVolumeID,
		},
	}
	fmt.Println("Waiting for volume to be ready...")
	waitUntilVolumeComplete := svc.WaitUntilVolumeAvailable(describeVolumeInput)
	if waitUntilVolumeComplete != nil {
		log.Fatal(waitUntilVolumeComplete)
	}
	fmt.Println("Volume is ready!")

	// Stop instance and unmount current volume
	stopInstancesInput := &ec2.StopInstancesInput{
		InstanceIds: []*string{
			instanceID,
		},
	}

	stopInstancesOutput, err := svc.StopInstances(stopInstancesInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Stopping Instance...")
	err = svc.WaitUntilInstanceStopped(describeInstanceInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Instance has been stopped!")
	fmt.Println(stopInstancesOutput.GoString())

	// Detach the Volume from the instance
	detachVolumeInput := &ec2.DetachVolumeInput{
		VolumeId: instanceVolumeID,
	}
	fmt.Println("Detaching Volume...")
	detachVolumeOutput, err := svc.DetachVolume(detachVolumeInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(detachVolumeOutput.GoString())
	err = svc.WaitUntilVolumeAvailable(&ec2.DescribeVolumesInput{
		VolumeIds: []*string{
			aws.String(*detachVolumeOutput.VolumeId),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Volume has been detached.")

	// Attach snapshot volume
	attachVolumeInput := &ec2.AttachVolumeInput{
		InstanceId: instanceID,
		VolumeId:   snapShotVolumeID,
		Device:     aws.String("/dev/xvda"),
	}
	attachVolumeOutput, err := svc.AttachVolume(attachVolumeInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Waiting for Volume to be attached")
	err = svc.WaitUntilVolumeInUse(&ec2.DescribeVolumesInput{
		VolumeIds: []*string{
			snapShotVolumeID,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Volume is now in use!")
	fmt.Println(attachVolumeOutput.GoString())

	// Start instance Up
	startInstanceInput := &ec2.StartInstancesInput{
		InstanceIds: []*string{
			instanceID,
		},
	}
	fmt.Println("Starting instance")
	startInstanceOutput, err := svc.StartInstances(startInstanceInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(*startInstanceOutput.StartingInstances[0].CurrentState.Name)
	err = svc.WaitUntilInstanceRunning(describeInstanceInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Instance has been started!")

	newIP, _ := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			instanceID,
		},
	})
	fmt.Println(*newIP.Reservations[0].Instances[0].PublicIpAddress)

}
