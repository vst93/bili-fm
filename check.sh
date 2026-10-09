#!/usr/bin/env bash
# 本地的 CI 门禁镜像：推送前跑一遍，别让 CI 替我们发现编译回归。
#
# 用法: ./check.sh          # 全量（六平台编译 + vet + test）
#       ./check.sh fast     # 只跑本机 vet + test

set -uo pipefail
cd "$(dirname "$0")/app"

export GOFLAGS=-mod=mod GOPROXY=off GOPRIVATE='*'

fail=0

run() {
    echo "── $*"
    if ! "$@"; then
        echo "✗ $* 失败"
        fail=1
    fi
}

echo "== go vet =="
run go vet ./...

echo "== go test =="
run go test -count=1 ./...

if [ "${1:-}" != "fast" ]; then
    for target in \
        linux/amd64 linux/arm64 \
        windows/amd64 windows/arm64 \
        darwin/amd64 darwin/arm64
    do
        goos=${target%%/*}
        goarch=${target##*/}
        echo "== build $goos/$goarch =="
        GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 \
            run go build -tags=nethttpomithttp2 ./...
    done
fi

if [ $fail -ne 0 ]; then
    echo "✗ 有失败项"
    exit 1
fi
echo "✓ 全部通过"
