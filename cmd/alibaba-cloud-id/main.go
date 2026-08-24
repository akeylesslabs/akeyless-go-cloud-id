package main

import (
	"fmt"
	"os"

	"github.com/akeylesslabs/akeyless-go-cloud-id/cloudprovider/alibaba"
)

func main() {
	cloudID, err := alibaba.GetCloudId()
	if err != nil {
		fmt.Fprintln(os.Stderr, "GetCloudId error:", err)
		os.Exit(1)
	}

	fmt.Println(cloudID)
}
