# syntax=docker/dockerfile:1.7
ARG GO_VERSION=1.26.8
FROM --platform=${TARGETPLATFORM} golang:${GO_VERSION}-bookworm AS base

ARG USER_ID=1000
ARG USER_NAME=eightaugusto
ENV HOME="/home/${USER_NAME}" \
    GOPATH="/home/${USER_NAME}/go" \
    GOMODCACHE="/home/${USER_NAME}/go/pkg/mod" \
    GOCACHE="/home/${USER_NAME}/.cache/go-build" \
    GOTMPDIR="/home/${USER_NAME}/.cache/go-tmp" \
    GOTOOLCHAIN=local \
    PATH="/home/${USER_NAME}/go/bin:/usr/local/go/bin:${PATH}"
RUN groupadd --gid "${USER_ID}" "${USER_NAME}" && \
    useradd --no-log-init --uid "${USER_ID}" --gid "${USER_NAME}" --create-home --shell /bin/bash "${USER_NAME}" && \
    mkdir -p "${GOMODCACHE}" "${GOCACHE}" "${GOTMPDIR}" "${GOPATH}/bin" "${HOME}/.cache/osxcross" && \
    chown -R "${USER_NAME}:${USER_NAME}" "${HOME}"

FROM base AS linux
RUN test "$(dpkg --print-architecture)" = amd64 && \
    apt-get update && apt-get install -y --no-install-recommends \
        file libgl1-mesa-dev libwayland-dev libxkbcommon-dev xorg-dev && \
    rm -rf /var/lib/apt/lists/*
USER ${USER_NAME}
WORKDIR ${HOME}
RUN go version

FROM base AS windows
RUN test "$(dpkg --print-architecture)" = amd64 && \
    apt-get update && apt-get install -y --no-install-recommends \
        binutils-mingw-w64-x86-64 file gcc-mingw-w64-x86-64 && \
    rm -rf /var/lib/apt/lists/*
USER ${USER_NAME}
WORKDIR ${HOME}
RUN go version

FROM base AS mac
ARG LLVM_VERSION=21
ARG MACOS_MINIMUM_VERSION=12.0
ARG OSXCROSS_REVISION=27d21e4977c9751d01199c7a226a6faf494c3dd9
ENV PATH="/usr/lib/llvm-${LLVM_VERSION}/bin:${PATH}" \
    MACOSX_DEPLOYMENT_TARGET=${MACOS_MINIMUM_VERSION}
# The Go image already contains CA certificates; apt keeps TLS verification on.
ADD --chmod=0644 https://apt.llvm.org/llvm-snapshot.gpg.key /usr/share/keyrings/apt.llvm.org.asc
RUN test "$(dpkg --print-architecture)" = arm64 && \
    echo "deb [arch=arm64 signed-by=/usr/share/keyrings/apt.llvm.org.asc] https://apt.llvm.org/bookworm/ llvm-toolchain-bookworm-${LLVM_VERSION} main" \
        > /etc/apt/sources.list.d/llvm.list && \
    apt-get update && apt-get install -y --no-install-recommends \
        clang-${LLVM_VERSION} cpio file lld-${LLVM_VERSION} llvm-${LLVM_VERSION} patch xz-utils && \
    rm -rf /var/lib/apt/lists/* && \
    ln -s "/usr/bin/clang-${LLVM_VERSION}" /usr/local/bin/clang && \
    ln -s "/usr/bin/clang++-${LLVM_VERSION}" /usr/local/bin/clang++ && \
    ln -s "/usr/bin/ld64.lld-${LLVM_VERSION}" /usr/local/bin/ld64.lld && \
    ln -s "/usr/bin/llvm-lipo-${LLVM_VERSION}" /usr/local/bin/llvm-lipo && \
    mkdir -p "${HOME}/tools/osxcross-source" && \
    git init --initial-branch=master "${HOME}/tools/osxcross-source" && \
    git -C "${HOME}/tools/osxcross-source" remote add origin https://github.com/tpoechtrager/osxcross.git && \
    git -C "${HOME}/tools/osxcross-source" fetch --depth=1 origin "${OSXCROSS_REVISION}" && \
    git -C "${HOME}/tools/osxcross-source" checkout --detach FETCH_HEAD && \
    test "$(git -C "${HOME}/tools/osxcross-source" rev-parse HEAD)" = "${OSXCROSS_REVISION}" && \
    rm -rf "${HOME}/tools/osxcross-source/.git" && \
    chown -R "${USER_NAME}:${USER_NAME}" "${HOME}/tools"
USER ${USER_NAME}
WORKDIR ${HOME}
RUN go version
