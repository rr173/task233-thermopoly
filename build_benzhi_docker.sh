#!/usr/bin/env bash
# build_benzhi_docker.sh —— 评测构建脚本
# 用法: bash build_benzhi_docker.sh <镜像名> <平台>
# 示例: bash build_benzhi_docker.sh my-project linux/amd64
set -euo pipefail

IMAGE_NAME="${1:-my-project}"
PLATFORM="${2:-linux/amd64}"
TAG="$(echo "${IMAGE_NAME}" | tr ':' '-')-${PLATFORM//\//-}"

echo ">> building ${IMAGE_NAME} for ${PLATFORM}"
docker buildx build \
  --platform "${PLATFORM}" \
  --load \
  -t "${TAG}" \
  -f benzhi.Dockerfile \
  .

echo ">> running smoke-test on ${TAG}"
docker run --rm --platform "${PLATFORM}" "${TAG}" --smoke-test
echo ">> ${PLATFORM} build + smoke-test OK"
