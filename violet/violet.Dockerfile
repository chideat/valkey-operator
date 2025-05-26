FROM build-harbor.alauda.cn/middleware/valkey-operator-violet-base:latest as violet

WORKDIR /workspace

COPY violet_build.sh ./

ARG TAG
ARG LEVEL

RUN set -ex; \
    ./violet_build.sh TAG=${TAG} LEVEL=${LEVEL}

FROM build-harbor.alauda.cn/ops/alpine:latest

COPY --from=violet /workspace/violet_build.sh .
