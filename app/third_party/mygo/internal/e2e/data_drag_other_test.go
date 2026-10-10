//go:build !darwin && !(windows && (amd64 || arm64))

package e2e

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

func dropNativeData(*mygo.Window, float64, float64, transfer.Data, transfer.Operation) (transfer.Operation, bool, bool) {
	return transfer.None, false, false
}
