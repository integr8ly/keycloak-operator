package k8s

import "github.com/operator-framework/operator-sdk/pkg/sdk"

type Cruder struct {
}

func (c Cruder) Create(o sdk.Object) error {
	return sdk.Create(o)
}
