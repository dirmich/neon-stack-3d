#!/usr/bin/env bash
set -e

# 맥(Apple Silicon)에서 빌드하여 리눅스 서버에 배포할 때를 대비해 기본 플랫폼을 linux/amd64로 설정합니다.
PLATFORM="${PLATFORM:-linux/amd64}"

FRONTEND_IMAGE="dirmich/neonstack:latest"
BACKEND_IMAGE="dirmich/neonstackb:latest"

echo "=========================================="
echo "🚀 Docker 이미지 빌드 및 푸시 시작 (Platform: ${PLATFORM})"
echo "=========================================="

echo ""
echo "📦 [1/4] 프론트엔드 이미지 빌드 중: ${FRONTEND_IMAGE}"
docker build --platform "${PLATFORM}" -t "${FRONTEND_IMAGE}" .

echo ""
echo "📦 [2/4] 백엔드(Go + Rust) 이미지 빌드 중: ${BACKEND_IMAGE}"
docker build --platform "${PLATFORM}" -f backend/Dockerfile -t "${BACKEND_IMAGE}" .

echo ""
echo "=========================================="
echo "✅ 모든 이미지 빌드 성공! Docker Hub로 푸시를 진행합니다."
echo "=========================================="

echo ""
echo "📤 [3/4] 프론트엔드 이미지 푸시 중: ${FRONTEND_IMAGE}"
docker push "${FRONTEND_IMAGE}"

echo ""
echo "📤 [4/4] 백엔드 이미지 푸시 중: ${BACKEND_IMAGE}"
docker push "${BACKEND_IMAGE}"

echo ""
echo "=========================================="
echo "🐙 Git 커밋 및 푸시 진행"
echo "=========================================="
VERSION=$(node -p "require('./package.json').version" 2>/dev/null || echo "deploy")
git add .
if git diff --staged --quiet; then
  echo "ℹ️ 커밋할 변경사항이 없습니다."
else
  git commit -m "chore: release v${VERSION} & deploy setup"
  git push
fi

echo ""
echo "=========================================="
echo "🎉 모든 이미지 푸시 및 Git 푸시가 완료되었습니다!"
echo "=========================================="
