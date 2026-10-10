//go:build !(windows && (amd64 || arm64)) && !darwin && !(linux && (amd64 || arm64))

package text

func newEngine() engine { return &stubEngine{} }
