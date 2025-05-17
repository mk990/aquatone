#!/bin/bash
set -e

# Modern build script for Aquatone

# Define colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Print banner
echo -e "${BLUE}Building Aquatone...${NC}"

# Set up version from git or default
VERSION=$(git describe --tags 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDTIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo -e "${GREEN}Version:${NC} $VERSION"
echo -e "${GREEN}Commit:${NC} $COMMIT"
echo -e "${GREEN}Build time:${NC} $BUILDTIME"

# Ensure output directory exists
mkdir -p build

# Clean up previous builds
rm -rf build/* 2>/dev/null || true

# Platforms to build for
platforms=("windows/amd64" "windows/386" "darwin/amd64" "darwin/arm64" "linux/amd64" "linux/386" "linux/arm" "linux/arm64")

for platform in "${platforms[@]}"; do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}

    output_name="aquatone"
    if [ $GOOS = "windows" ]; then
        output_name+=".exe"
    fi

    echo -e "${BLUE}Building for ${GOOS}/${GOARCH}...${NC}"

    # Modern env-var based build with Go modules
    export GOOS=$GOOS
    export GOARCH=$GOARCH
    export CGO_ENABLED=0

    go_build_cmd="go build -trimpath -o build/aquatone_${GOOS}_${GOARCH}/${output_name} -ldflags=\"-s -w -X github.com/mk990/aquatone/core.Version=$VERSION -X github.com/mk990/aquatone/core.CommitHash=$COMMIT -X github.com/mk990/aquatone/core.BuildTime=$BUILDTIME\""

    # Use eval to properly handle the string with quotes
    eval "$go_build_cmd"

    if [ $? -ne 0 ]; then
        echo -e "${RED}Error building for ${GOOS}/${GOARCH}${NC}"
        exit 1
    fi

    # Create zip archive
    cd build
    zip -q -r "aquatone_${GOOS}_${GOARCH}.zip" "aquatone_${GOOS}_${GOARCH}/"
    cd ..

    echo -e "${GREEN}✓ Done!${NC}"
done

echo -e "${GREEN}All builds completed successfully!${NC}"
