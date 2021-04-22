package akeyless_go_cloud_id

import "github.com/akeylesslabs/akeyless-go-cloud-id/cloudprovider/aws"

// GetCloudId returns AWS cloud identity.
//
// Deprecated: use cloud-specific sub-package instead, for example
// github.com/akeylesslabs/akeyless-go-cloud-id/cloudprovider/aws
var GetCloudId = aws.GetCloudId
