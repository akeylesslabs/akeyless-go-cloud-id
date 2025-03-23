package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type captureSingedRequest struct {
	req *smithyhttp.Request
}

func (c *captureSingedRequest) ID() string {
	return "captureSingedRequest"
}
func (c *captureSingedRequest) HandleFinalize(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (
	out middleware.FinalizeOutput, md middleware.Metadata, err error,
) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, md, fmt.Errorf("unrecognized transport type %T", in.Request)
	}
	c.req = req

	// We don't want to actually do the request, we just want to capture the request and leave, so we return "empty" output
	return middleware.FinalizeOutput{Result: &sts.GetCallerIdentityOutput{}}, md, nil
}
