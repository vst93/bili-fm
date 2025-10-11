#!/bin/bash

# 输入版本号
read -p "请输入版本号（例如 x.x.x）: " VERSION

# 定义平台和架构
PLATFORMS=("darwin/amd64" "darwin/arm64" "windows/amd64" "linux/amd64")

# 标志是否为第一次循环
FIRST_RUN=true

# 遍历平台并构建
for PLATFORM in "${PLATFORMS[@]}"; do
  # 提取平台名称和架构
  OS=$(echo $PLATFORM | cut -d'/' -f1)
  ARCH=$(echo $PLATFORM | cut -d'/' -f2)

  if [ "$OS" == "darwin" ]; then
    OS="mac"
  fi

  # 设置输出文件名
  OUTPUT_NAME="bili-FM"
  ZIP_NAME="bili-FM_${VERSION}_${OS}_${ARCH}.zip"

  # 构建应用程序
  echo "正在构建 ${OS} ${ARCH} 平台..."

  # 如果是第一次循环，加上 -clean 参数
  if [ "$FIRST_RUN" = true ]; then
    wails build -platform $PLATFORM -clean -s 
    FIRST_RUN=false
  else
    wails build -platform $PLATFORM -s -skipbindings
  fi

  # 检查构建是否成功
  if [ $? -eq 0 ]; then
    echo "构建成功！正在打包为 ${ZIP_NAME}..."

    # 进入构建输出目录
    cd build/bin || { echo "无法进入 build/bin 目录"; exit 1; }

    # 打包为 ZIP 文件
    if [ "$OS" == "windows" ]; then
      zip -r ./${ZIP_NAME} ${OUTPUT_NAME}.exe
    elif [ "$OS" == "darwin" ]; then
      zip -r ./${ZIP_NAME} ${OUTPUT_NAME}.app
    elif [ "$OS" == "mac" ]; then
      zip -r ./${ZIP_NAME} ${OUTPUT_NAME}.app
    else
      zip -r ./${ZIP_NAME} ${OUTPUT_NAME}
    fi

    # 返回上一级目录
    cd ../..

    # 检查打包是否成功
    if [ $? -eq 0 ]; then
      echo "打包完成: ${ZIP_NAME}"
    else
      echo "打包失败: ${OS} ${ARCH}"
      exit 1
    fi
  else
    echo "构建失败: ${OS} ${ARCH}"
    exit 1
  fi
done

echo "所有平台构建和打包完成！"
